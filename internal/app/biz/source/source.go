package source

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	embedimpl "github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding/impl"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	vschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	einoembed "github.com/cloudwego/eino/components/embedding"
)

var ErrSourceNotFound = errors.New("source not found")

type Biz struct {
	objectStorage  storage.Storage
	sourceStore    dal.SourceStore
	sourceDocStore vectordal.SourceDocStore

	embedder            einoembed.Embedder
	embedBatchSize      int
	embedMaxConcurrency int

	docHandlers map[model.SourceKind]convertdoc.Handler
}

func New(
	objectStorage storage.Storage,
	sourceStore dal.SourceStore,
	sourceDocStore vectordal.SourceDocStore,
) *Biz {
	embedder, err := embedimpl.New(
		context.Background(),
		conf.Global().Embedding.Type,
		&conf.Global().Embedding,
	)
	if err != nil {
		panic(err)
	}

	hc := convertdoc.HandlerConfig{
		ChunkSize:   conf.Global().Chunking.Size,
		OverlapSize: conf.Global().Chunking.OverlapSize,
	}
	if hc.OverlapSize == 0 || hc.OverlapSize > hc.ChunkSize {
		hc.OverlapSize = int(float64(hc.ChunkSize) * 0.15)
	}

	b := &Biz{
		objectStorage:       objectStorage,
		sourceStore:         sourceStore,
		sourceDocStore:      sourceDocStore,
		embedder:            embedder,
		embedBatchSize:      conf.Global().Embedding.BatchSize,
		embedMaxConcurrency: conf.Global().Embedding.MaxConcurrency,
		docHandlers: map[model.SourceKind]convertdoc.Handler{
			model.SourceKindText: convertdoc.NewTextHandler(hc),
			model.SourceKindUrl:  convertdoc.NewUrlHandler(hc),
			model.SourceKindFile: convertdoc.NewFileObjectHandler(hc, objectStorage),
		},
	}

	return b
}

func (b *Biz) GetSource(ctx context.Context, id uuid.UUID) (*model.Source, error) {
	source, err := b.sourceStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, ErrSourceNotFound
		}
		return nil, errors.WithMessage(err, "store get source failed")
	}

	return model.NewSourceFrom(source), nil
}

func (b *Biz) CountSourcesByNotebook(
	ctx context.Context,
	notebookId uuid.UUID,
) (int64, error) {
	count, err := b.sourceStore.CountByNotebookId(ctx, notebookId)
	if err != nil {
		return 0, errors.WithMessage(err, "store count sources failed")
	}

	return count, nil
}

func (b *Biz) ListSourcesByNotebook(
	ctx context.Context,
	notebookId uuid.UUID,
	limit, offset int,
) ([]*model.SourceWithContent, error) {
	rows, err := b.sourceStore.ListByNotebookId(ctx, notebookId, limit, offset)
	if err != nil {
		return nil, errors.WithMessage(err, "store list sources failed")
	}

	sources := make([]*model.Source, 0, len(rows))
	for i := range rows {
		row := rows[i]
		sources = append(sources, model.NewSourceFrom(row))
	}

	sourcesWithContents := make([]*model.SourceWithContent, 0, len(sources))
	for _, source := range sources {
		sc, err := model.NewSourceWithContent(source)
		if err != nil {
			slog.ErrorContext(ctx, "new source with content failed", slog.Any("err", err))
			return nil, errors.WithMessagef(err, "new source with content failed, source_id=%s", source.Id)
		}

		// if source is file, replace storekey with url link
		if sc.Kind.IsFile() {
			req := &storage.PresignedGetObjectRequest{
				Key:         sc.ContentFile.StoreKey,
				Inline:      true,
				ContentType: previewResponseContentType(sc.ContentFile.Format),
			}
			resp, err := b.objectStorage.PresignedGetObject(ctx, req)
			if err != nil {
				// we don't need to break for error here
				slog.ErrorContext(ctx, "get file source object url failed", slog.Any("err", err))
			} else {
				sc.ContentFile.Url = resp.Url
			}
		}

		sourcesWithContents = append(sourcesWithContents, sc)
	}

	return sourcesWithContents, nil
}

func previewResponseContentType(mimeType string) string {
	switch mimeType {
	case model.MimeTypeText:
		return "text/plain; charset=utf-8"
	case model.MimeTypeMarkdown:
		return "text/markdown; charset=utf-8"
	default:
		return mimeType
	}
}

type CreateSourceCommand struct {
	NotebookId  uuid.UUID
	Kind        model.SourceKind
	TextContent string
	UrlContent  *url.URL
}

func (b *Biz) CreateSource(ctx context.Context, cmd *CreateSourceCommand) (*model.Source, error) {
	newSource, err := buildNewSource(ctx, cmd)
	if err != nil {
		return nil, errors.WithMessagef(err, "build new source failed")
	}

	err = b.sourceStore.Create(ctx, newSource.To())
	if err != nil {
		return nil, errors.WithMessage(err, "store create source failed")
	}

	return newSource, nil
}

func (b *Biz) UpdateStatus(ctx context.Context, id uuid.UUID, status model.SourceStatus) error {
	err := b.sourceStore.UpdateStatus(ctx, id, status.String())
	if err != nil {
		return errors.WithMessagef(err, "store update source status failed, id=%s", id)
	}

	return nil
}

func (b *Biz) DeleteSource(ctx context.Context, sourceId uuid.UUID) error {
	source, err := b.GetSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, ErrSourceNotFound) || errors.Is(err, errors.ErrNoRecord) {
			return nil
		}
		return errors.WithMessagef(err, "get source failed before deleting, id=%s", sourceId)
	}

	var fileStoreKey string
	if source.KindFile() && len(source.Content) != 0 {
		fileContent := model.FileSourceContent{}
		err = fileContent.From(source.Content)
		if err != nil {
			slog.WarnContext(ctx, "ignore parse source object key due invalid file source content",
				slog.String("source_id", sourceId.String()),
				slog.Any("err", errors.Wrapf(
					errors.ErrSerde,
					"unmarshal file source content failed before deleting source, source_id=%s",
					sourceId,
				)))
		} else {
			fileStoreKey = fileContent.StoreKey
		}
	}

	err = b.sourceStore.DeleteById(ctx, sourceId)
	if err != nil {
		return errors.WithMessagef(err, "store delete source failed, id=%s", sourceId)
	}

	err = b.sourceDocStore.BatchDelete(ctx, &vschema.SourceDocBatchDeleteParams{
		NotebookId: source.NotebookId.String(),
		SourceId:   []string{source.Id.String()},
	})
	if err != nil {
		return errors.WithMessagef(err, "delete source docs failed, source_id=%s", sourceId)
	}

	if fileStoreKey != "" {
		err = b.objectStorage.DeleteObject(ctx, &storage.DeleteObjectRequest{
			Key: fileStoreKey,
		})
		if err != nil {
			slog.WarnContext(ctx, "ignore delete source object failure",
				slog.String("source_id", sourceId.String()),
				slog.String("key", fileStoreKey),
				slog.Any("err", err),
			)
		}
	}

	return nil
}

type UpdateContentCommand struct {
	Id          uuid.UUID
	Content     []byte
	Status      model.SourceStatus
	DisplayName string
}

func (b *Biz) UpdateContent(ctx context.Context, cmd *UpdateContentCommand) error {
	err := b.sourceStore.Update(ctx, &schema.SourceUpdateParams{
		Id:          cmd.Id,
		Content:     cmd.Content,
		Status:      cmd.Status.String(),
		DisplayName: cmd.DisplayName,
		UpdatedAt:   time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessagef(err, "store update source content failed, id=%s", cmd.Id)
	}

	return nil
}

// prepare source
func (b *Biz) PrepareSource(ctx context.Context, sourceId uuid.UUID) error {
	source, err := b.GetSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, ErrSourceNotFound) {
			return nil
		}

		return errors.WithMessagef(err, "get source failed, id=%s", sourceId)
	}

	var (
		notebookIdStr = source.NotebookId.String()
		sourceIdStr   = sourceId.String()
	)

	handler, ok := b.docHandlers[source.Kind]
	if !ok {
		return errors.ErrParams.Msgf("can not embed source for kind %s", source.Kind)
	}

	result, err := handler.Handle(ctx, source)
	if err != nil {
		return errors.WithMessagef(err, "embed source failed")
	}

	texts := make([]string, 0, len(result.Docs))
	for _, doc := range result.Docs {
		texts = append(texts, doc.Content)
	}

	slog.DebugContext(ctx, "embedding source docs",
		slog.Int("text_size", len(texts)),
		slog.Int("batch_size", b.embedBatchSize),
		slog.Int("max_concurrency", b.embedMaxConcurrency),
		slog.String("source_id", sourceIdStr))

	embeddings, err := batch.ParallelMap(
		ctx,
		texts,
		b.embedBatchSize,
		b.embedMaxConcurrency,
		func(ctx context.Context, bt []string) ([][]float64, error) {
			return b.embedder.EmbedStrings(ctx, bt)
		},
	)
	if err != nil {
		return errors.WithMessagef(err, "embed docs failed")
	}
	if len(embeddings) != len(texts) {
		return errors.Wrapf(
			errors.ErrSerde,
			"embed result count mismatch, expected=%d, actual=%d",
			len(texts),
			len(embeddings),
		)
	}

	docs := make([]*vschema.SourceDoc, len(result.Docs))
	for i, doc := range result.Docs {
		embedding := embeddings[i]
		embedding32 := make([]float32, len(embedding))
		for j, v := range embedding {
			embedding32[j] = float32(v)
		}
		docs[i] = &vschema.SourceDoc{
			Id:         doc.ID,
			NotebookId: notebookIdStr,
			SourceId:   sourceIdStr,
			Content:    doc.Content,
			Owner:      source.OwnerId,
			Embedding:  embedding32,
		}
	}

	err = b.sourceDocStore.BatchInsert(ctx, docs)
	if err != nil {
		return errors.WithMessagef(err, "insert source docs failed")
	}

	return nil
}

type CheckSourceIdsQuery struct {
	NotebookId uuid.UUID
	SourceIds  []uuid.UUID
}

// 检查source ids是否存在且属于notebookid
func (b *Biz) CheckSourceIds(
	ctx context.Context,
	query *CheckSourceIdsQuery,
) ([]uuid.UUID, error) {
	qids := pslices.Unique(query.SourceIds)
	rows, err := b.sourceStore.ListByNotebookIdAndIds(
		ctx,
		query.NotebookId,
		qids,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "store list sources failed")
	}

	// 返回query ids中出现在rows中的ids
	existSourceIds := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		existSourceIds = append(existSourceIds, row.Id)
	}

	return existSourceIds, nil
}

func buildNewSource(ctx context.Context, cmd *CreateSourceCommand) (*model.Source, error) {
	var (
		sourceId = uuid.NewV7()
		ownerId  = pkgcontext.GetUserId(ctx)
		source   = &model.Source{
			Id:         sourceId,
			NotebookId: cmd.NotebookId,
			Kind:       cmd.Kind,
			Status:     model.SourceStatusInited, // all new sources are inited
			OwnerId:    ownerId,
			UpdatedAt:  time.Now().UnixMilli(),
		}

		err error
	)

	switch cmd.Kind {
	case model.SourceKindText:
		ts := model.TextSourceContent{Text: cmd.TextContent}
		source.Content, err = sonic.Marshal(&ts)
		source.DisplayName = truncateRunes(cmd.TextContent, 32)
	case model.SourceKindUrl:
		us := model.UrlSourceContent{Url: cmd.UrlContent.String()}
		source.Content, err = sonic.Marshal(&us)
		source.DisplayName = us.Url
	case model.SourceKindFile:
		// file source inited with empty content
		source.Content = nil
		source.DisplayName = ""
	default:
		return nil, errors.ErrParams.Msgf("invalid source kind: %s", cmd.Kind)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "marshal source failed, kind=%s, source_id=%s", cmd.Kind, sourceId)
	}

	return source, err
}

func truncateRunes(input string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	return string(runes[:max])
}
