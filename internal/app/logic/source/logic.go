package source

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/textgen/summarizer"

	// "github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/openai"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	// "github.com/gonotelm-lab/gonotelm/pkg/mutex"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	// "github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

// mq topic names
const (
	TopicSourcePreparation = "gonotelm.source.preparation"
)

// consumer group id
const (
	SourcePreparationConsumerGroup = "gonotelm.source.preparation.group"
	maxUploadFileSizeBytes         = 100 * 1024 * 1024
)

type sourceEventMessage struct {
	Id         uuid.UUID          `json:"id"`
	NotebookId uuid.UUID          `json:"notebook_id"`
	Kind       model.SourceKind   `json:"kind"`
	Status     model.SourceStatus `json:"status"`
	UserId     string             `json:"user_id"`
}

type Logic struct {
	rootCtx context.Context
	redis   redis.UniversalClient

	objectStorage storage.Storage

	notebookBiz  *biznotebook.Biz
	sourceBiz    *bizsource.Biz
	mqFactory    *mq.MQ
	prepProducer mq.Producer
	prepConsumer mq.Consumer

	llmGateway *openai.Gateway
	summarizer summarizer.Summarizer
	prompt     *bizprompt.Prompt

	wg sync.WaitGroup
}

func (l *Logic) Close(ctx context.Context) {
	if l.prepConsumer != nil {
		if err := l.prepConsumer.Close(ctx); err != nil {
			slog.WarnContext(ctx, "close source prep consumer failed", slog.Any("err", err))
		}
	}
	if l.prepProducer != nil {
		if err := l.prepProducer.Close(ctx); err != nil {
			slog.WarnContext(ctx, "close source prep producer failed", slog.Any("err", err))
		}
	}

	l.wg.Wait()
}

func MustNewLogic(
	rootCtx context.Context,
	mqFactory *mq.MQ,
	redisClient redis.UniversalClient,
	objectStorage storage.Storage,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	llmGateway *openai.Gateway,
	prompt *bizprompt.Prompt,
) *Logic {
	sl := &Logic{
		rootCtx:       rootCtx,
		objectStorage: objectStorage,
		notebookBiz:   notebookBiz,
		sourceBiz:     sourceBiz,
		mqFactory:     mqFactory,
		llmGateway:    llmGateway,
		redis:         redisClient,
		prompt:        prompt,
	}

	summarizer := summarizer.NewWithOption(llmGateway,
		summarizer.SummarizeOption{
			Provider: conf.Global().Logic.Source.ModelProvider,
			Model:    conf.Global().Logic.Source.Model,
		},
		sl.prompt)

	sl.summarizer = summarizer

	return sl
}

type CreateSourceParams struct {
	NotebookId uuid.UUID
	Kind       model.SourceKind // text, url, file
	Text       string           // text kind
	Url        *url.URL         // url kind
}

func (l *Logic) GetSource(
	ctx context.Context,
	id uuid.UUID,
) (*model.Source, error) {
	source, err := l.sourceBiz.GetSource(ctx, id)
	if err != nil {
		return nil, wrapGetSourceError(err, id)
	}

	return source, nil
}

func (l *Logic) UpdateSourceTitle(
	ctx context.Context,
	sourceId uuid.UUID,
	title string,
) error {
	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		return errors.ErrParams.Msg("source title is empty")
	}

	source, err := l.GetSource(ctx, sourceId)
	if err != nil {
		return errors.WithMessagef(err, "get source failed, source_id=%s", sourceId)
	}

	userId := pkgcontext.GetUserId(ctx)
	if source.OwnerId != userId {
		return errors.ErrPermission.Msg("source access denied")
	}

	if source.Title == nextTitle {
		return nil
	}

	err = l.sourceBiz.UpdateTitle(ctx, sourceId, nextTitle)
	if err != nil {
		return errors.WithMessagef(err, "update source title failed, source_id=%s", sourceId)
	}

	return nil
}

type GetSourceDocParams struct {
	SourceId uuid.UUID
	DocId    string
}

type GetSourceDocResult struct {
	SourceId    string
	SourceTitle string
	Doc         *model.SourceDoc
}

type BatchGetSourceDocsParams struct {
	SourceId uuid.UUID
	DocIds   []string
}

func (l *Logic) GetSourceDoc(
	ctx context.Context,
	params *GetSourceDocParams,
) (*GetSourceDocResult, error) {
	userId := pkgcontext.GetUserId(ctx)

	source, err := l.GetSource(ctx, params.SourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source failed, source_id=%s", params.SourceId)
	}
	if source.OwnerId != userId {
		return nil, errors.ErrPermission.Msg("source access denied")
	}

	doc, err := l.sourceBiz.GetSourceDoc(ctx,
		&bizsource.GetSourceDocQuery{
			NotebookId: source.NotebookId,
			SourceId:   source.Id,
			DocId:      params.DocId,
			Populate:   true,
		})
	if err != nil {
		return nil, errors.WithMessagef(err, "get source doc failed, source_id=%s, doc_id=%s", source.Id, params.DocId)
	}

	return &GetSourceDocResult{
		SourceId:    source.Id.String(),
		SourceTitle: source.Title,
		Doc:         doc,
	}, nil
}

type BatchGetSourceDocsResult struct {
	SourceId    string
	SourceTitle string
	Docs        []*model.SourceDoc
}

func (l *Logic) BatchGetSourceDocs(
	ctx context.Context,
	params *BatchGetSourceDocsParams,
) (*BatchGetSourceDocsResult, error) {
	userId := pkgcontext.GetUserId(ctx)

	source, err := l.GetSource(ctx, params.SourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source failed, source_id=%s", params.SourceId)
	}
	if source.OwnerId != userId {
		return nil, errors.ErrPermission.Msg("source access denied")
	}

	docs, err := l.sourceBiz.BatchGetSourceDocs(ctx,
		&bizsource.BatchGetSourceDocsQuery{
			NotebookId: source.NotebookId,
			SourceId:   source.Id,
			DocIds:     params.DocIds,
			Populate:   true,
		})
	if err != nil {
		return nil, errors.WithMessagef(err, "batch get source docs failed, source_id=%s", source.Id)
	}

	return &BatchGetSourceDocsResult{
		SourceId:    source.Id.String(),
		SourceTitle: source.Title,
		Docs:        docs,
	}, nil
}

type UploadSourceParams struct {
	SourceId uuid.UUID
	Filename string
	MimeType string
	Size     int64
	Md5      string
}

type UploadSourceResult struct {
	Url     string            `json:"url"`
	Method  string            `json:"method"`
	Forms   map[string]string `json:"forms"`
	Headers map[string]string `json:"headers"`
}

func (l *Logic) DeleteNotebookSources(
	ctx context.Context,
	notebookId uuid.UUID,
) error {
	err := l.sourceBiz.DeleteNotebookSources(ctx, notebookId)
	if err != nil {
		return errors.WithMessagef(err, "delete notebook sources failed, notebook_id=%s", notebookId)
	}

	return nil
}

type GetFullSourceParams struct {
	Download bool
}
