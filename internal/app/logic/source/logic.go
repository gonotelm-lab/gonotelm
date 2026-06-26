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
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/mq"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/mutex"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
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

	llmGateway *gateway.Gateway
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
	infras *infra.Instances,
	objectStorage storage.Storage,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	llmGateway *gateway.Gateway,
	prompt *bizprompt.Prompt,
) *Logic {
	sl := &Logic{
		rootCtx:       rootCtx,
		objectStorage: objectStorage,
		notebookBiz:   notebookBiz,
		sourceBiz:     sourceBiz,
		mqFactory:     infras.MQ,
		llmGateway:    llmGateway,
		redis:         infras.Redis,
		prompt:        prompt,
	}

	summarizer := summarizer.NewWithOption(llmGateway,
		summarizer.SummarizeOption{
			Provider: conf.Global().Logic.Source.ModelProvider,
			Model:    conf.Global().Logic.Source.Model,
		})

	sl.summarizer = summarizer
	sl.mustInitMsgQueue()

	return sl
}

func (l *Logic) mustInitMsgQueue() {
	if l.mqFactory == nil || l.mqFactory.NewProducer == nil || l.mqFactory.NewConsumer == nil {
		panic("message queue is not initialized")
	}

	// producer
	l.prepProducer = l.mqFactory.NewProducer()
	// consumer
	l.prepConsumer = l.mqFactory.NewConsumer(TopicSourcePreparation, SourcePreparationConsumerGroup)
	l.prepConsumer.Subscribe(l.rootCtx, TopicSourcePreparation, l.handleSourceEventMessage)
}

type CreateSourceParams struct {
	NotebookId uuid.UUID
	Kind       model.SourceKind // text, url, file
	Text       string           // text kind
	Url        *url.URL         // url kind
}

func (l *Logic) CreateSource(
	ctx context.Context,
	params *CreateSourceParams,
) (*model.Source, error) {
	userId := pkgcontext.GetUserId(ctx)
	// check if notebook exists
	_, err := l.notebookBiz.GetNotebook(ctx, params.NotebookId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, errors.ErrParams.Msgf("notebook not found, id=%s", params.NotebookId)
		}

		return nil, errors.WithMessagef(err, "get notebook failed, id=%s", params.NotebookId)
	}

	locker := mutex.NewRedisLock(l.redis, formatSourceCreateCacheKey(params.NotebookId))
	err = locker.LockContext(ctx)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrCache,
			"lock source create failed, notebook_id=%s, err=%v",
			params.NotebookId, err,
		)
	}
	defer locker.UnlockContext(ctx)

	// count
	count, err := l.sourceBiz.CountSourcesByNotebook(ctx, params.NotebookId)
	if err != nil {
		return nil, errors.WithMessagef(err, "count sources failed, notebook_id=%s", params.NotebookId)
	}
	if count >= constants.MaxSourceCountPerNotebook {
		return nil, errors.ErrParams.Msgf("source count exceeds max count, notebook_id=%s", params.NotebookId)
	}

	source, err := l.sourceBiz.CreateSource(
		ctx, &bizsource.CreateSourceCommand{
			NotebookId:  params.NotebookId,
			OwnerId:     userId,
			Kind:        params.Kind,
			TextContent: params.Text,
			UrlContent:  params.Url,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "create source failed")
	}

	if !source.KindFile() {
		// not file kind, we can do preparing immediately
		err = l.notifySourceEventMessage(ctx, source, false)
		if err != nil {
			return nil, errors.Wrap(err, "send source inserted message failed")
		}
	}

	return source, nil
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

func (l *Logic) UploadFileSource(
	ctx context.Context,
	params *UploadSourceParams,
) (*UploadSourceResult, error) {
	if params.Size > maxUploadFileSizeBytes {
		return nil, errors.ErrParams.Msg("file size too large")
	}
	if !model.SupportedFileMimeType(params.MimeType) {
		return nil, errors.ErrParams.Msgf("unsupported mime_type: %s", params.MimeType)
	}

	source, err := l.GetSource(ctx, params.SourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source failed, source_id=%s", params.SourceId)
	}

	if !checkSourceUploadable(source) {
		return nil, errors.ErrParams.Msgf("source is not uploadable, kind=%s, status=%s",
			source.Kind, source.Status)
	}

	storeKey := formatSourceStoreKey(params, source)
	policy, err := l.objectStorage.PresignedPostPolicy(ctx,
		&storage.PresignedPostPolicyRequest{
			Key:           storeKey,
			ContentType:   params.MimeType,
			ContentLength: params.Size,
			Filename:      params.Filename,
			Md5:           params.Md5,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "presigned post policy failed")
	}

	content := model.FileSourceContent{
		StoreKey: storeKey,
		Filename: params.Filename,
		Size:     params.Size,
		Md5:      params.Md5,
		Format:   params.MimeType,
	}
	// store file data as source content
	err = l.updateFileSourceContent(ctx, source, &content)
	if err != nil {
		return nil, errors.WithMessage(err, "store update source failed")
	}

	return &UploadSourceResult{
		Url:     policy.Url,
		Method:  policy.Method,
		Forms:   policy.Forms,
		Headers: policy.Headers,
	}, nil
}

// check source status and update if necessary
func (l *Logic) PollSourceStatus(
	ctx context.Context,
	sourceId uuid.UUID,
) (model.SourceStatus, error) {
	target, err := l.sourceBiz.GetSource(ctx, sourceId)
	if err != nil {
		// source may be deleted while frontend polling status; treat as terminal status.
		if errors.Is(err, bizsource.ErrSourceNotFound) || errors.Is(err, errors.ErrNoRecord) {
			return model.SourceStatusFailed, nil
		}
		return "", errors.WithMessagef(err, "get source failed, id=%s", sourceId)
	}

	if target.StatusReady() {
		return target.Status, nil
	}

	var status model.SourceStatus = target.Status
	if target.KindFile() {
		status, err = l.pollFileSourceStatus(ctx, target)
		// lazy update source status
		if err != nil {
			return "", errors.WithMessage(err, "poll source status failed")
		}
	}

	return status, nil
}

// 失败时可以重新构建来源
func (l *Logic) RetrySourcePreparation(
	ctx context.Context,
	sourceId uuid.UUID,
) error {
	source, err := l.GetSource(ctx, sourceId)
	if err != nil {
		return errors.WithMessagef(err, "get source failed, id=%s", sourceId)
	}

	if !source.StatusFailed() {
		return errors.ErrParams.Msg("no need to retry")
	}

	err = l.sourceBiz.UpdateStatus(ctx, source.Id, model.SourceStatusPreparing)
	if err != nil {
		return errors.WithMessage(err, "update source status to preparing failed")
	}
	source.Status = model.SourceStatusPreparing

	// do retry by re-sending the source event message
	err = l.notifySourceEventMessage(ctx, source, true)
	if err != nil {
		rollbackErr := l.sourceBiz.UpdateStatus(ctx, source.Id, model.SourceStatusFailed)
		if rollbackErr != nil {
			slog.WarnContext(ctx, "rollback source status after retry notify failure failed",
				slog.String("source_id", source.Id.String()),
				slog.Any("err", rollbackErr))
		}
		return errors.WithMessage(err, "notify source preparing failed")
	}

	return nil
}

func (l *Logic) DeleteSource(ctx context.Context, sourceId uuid.UUID) error {
	err := l.sourceBiz.DeleteSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, bizsource.ErrSourceNotFound) {
			return nil
		}

		return errors.WithMessagef(err, "delete source failed, id=%s", sourceId)
	}

	return nil
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

func (l *Logic) GetFullSource(
	ctx context.Context,
	sourceId uuid.UUID,
	params *GetFullSourceParams,
) (*model.FullSource, error) {
	source, err := l.sourceBiz.GetDecodedSource(
		ctx,
		sourceId,
		bizsource.WithContentRefUrl(true),
	)
	if err != nil {
		if errors.Is(err, bizsource.ErrSourceNotFound) {
			return nil, errors.ErrParams.Msgf("source not found, id=%s", sourceId)
		}

		return nil, errors.WithMessagef(err, "get source failed, id=%s", sourceId)
	}

	userId := pkgcontext.GetUserId(ctx)
	// check user permission
	if source.OwnerId != userId {
		return nil, errors.ErrPermission.Msg("source access denied")
	}

	if source.Status != model.SourceStatusReady {
		return nil, errors.ErrParams.Msgf("source is not ready, status=%s", source.Status)
	}

	fullSource := &model.FullSource{
		DecodedSource: source,
	}
	err = l.sourceBiz.BatchPopulateFullSources(
		ctx,
		[]*model.FullSource{fullSource},
		bizsource.WithForDownload(params.Download),
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "populate full source failed, source_id=%s", sourceId)
	}

	return fullSource, nil
}

func (l *Logic) GetSourceParsedTree(
	ctx context.Context,
	sourceId uuid.UUID,
) (*ParsedSourceDocTree, error) {
	source, err := l.GetSource(ctx, sourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source failed, source_id=%s", sourceId)
	}

	tree, err := l.sourceBiz.GetSourceDocTree(ctx, source.NotebookId, sourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source doc tree failed, source_id=%s", sourceId)
	}

	return buildParsedSourceDocTree(tree), nil
}

func (l *Logic) CheckSourceUserId(ctx context.Context, sourceId uuid.UUID) error {
	sourceUserId, err := l.sourceBiz.GetSourceUser(ctx, sourceId)
	if err != nil {
		return errors.WithMessagef(err, "get source user failed, source_id=%s", sourceId)
	}

	userId := pkgcontext.GetUserId(ctx)
	if sourceUserId != userId {
		return errors.ErrPermission.Msgf("source access denied, source_id=%s", sourceId)
	}

	return nil
}

func (l *Logic) pollFileSourceStatus(
	ctx context.Context,
	source *model.Source,
) (status model.SourceStatus, err error) {
	// check if file already uploaded
	fileSource := model.FileSourceContent{}
	status = source.Status

	if len(source.Content) != 0 { // normally empty content means inited status
		err = sonic.Unmarshal(source.Content, &fileSource)
		if err != nil {
			err = errors.Wrapf(errors.ErrSerde,
				"unmarshal file source failed, kind=%s, source_id=%s, err=%v",
				source.Kind, source.Id, err)
			return
		}
	}

	if !source.StatusUploading() {
		return source.Status, nil
	}

	// uploading status, check if file already uploaded
	_, err = l.objectStorage.StatObject(ctx, &storage.StatObjectRequest{
		Key: fileSource.StoreKey,
	})
	// TODO check if object is actually supported from mime type
	if err != nil {
		if !errors.Is(err, storage.ErrObjectNotFound) {
			err = errors.WithMessagef(err, "stat object failed, key=%s", fileSource.StoreKey)
			return
		}
		err = nil
	} else {
		// uploaded, make it preparing
		err = l.notifySourceEventMessage(ctx, source, false)
		if err != nil {
			err = errors.WithMessage(err, "notify source preparing failed")
			return
		}

		// update source status
		err = l.sourceBiz.UpdateStatus(ctx, source.Id, model.SourceStatusPreparing)
		if err != nil {
			err = errors.WithMessage(err, "update source status failed")
			return
		}
		status = model.SourceStatusPreparing
	}

	return
}

func (l *Logic) updateFileSourceContent(
	ctx context.Context,
	source *model.Source,
	fileContent *model.FileSourceContent,
) error {
	if source == nil {
		return errors.ErrParams.Msg("source is nil")
	}
	if fileContent == nil {
		return errors.ErrParams.Msg("file source content is nil")
	}

	content, err := sonic.Marshal(fileContent)
	if err != nil {
		return errors.WithMessage(err, "marshal file source failed")
	}

	return l.sourceBiz.UpdateContent(ctx, &bizsource.UpdateContentCommand{
		Id:      source.Id,
		Status:  model.SourceStatusUploading,
		Title:   fileContent.Filename,
		Content: content,
	})
}

func checkSourceUploadable(source *model.Source) bool {
	return source.KindFile() && source.StatusInited()
}
