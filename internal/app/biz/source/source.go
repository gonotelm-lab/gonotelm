package source

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	vecschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

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

	docConverters map[model.SourceKind]convertdoc.Handler
}

func New(
	objectStorage storage.Storage,
	sourceStore dal.SourceStore,
	sourceDocStore vectordal.SourceDocStore,
) (*Biz, error) {
	embedder, err := embedding.New(
		context.Background(),
		&conf.Global().Embedding,
		embedding.NewRedisCacher(cache.GetRedis()),
	)
	if err != nil {
		return nil, errors.WithMessage(err, "new embedder failed")
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
		docConverters: map[model.SourceKind]convertdoc.Handler{
			model.SourceKindText: convertdoc.NewTextHandler(hc),
			model.SourceKindUrl:  convertdoc.NewUrlHandler(hc),
			model.SourceKindFile: convertdoc.NewFileObjectHandler(hc, objectStorage),
		},
	}

	return b, nil
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

type CreateSourceCommand struct {
	NotebookId  uuid.UUID
	OwnerId     string
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

	err = b.sourceDocStore.BatchDelete(ctx, &vecschema.SourceDocBatchDeleteParams{
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

// 准备数据源
// 包含chunk + embedding的索引过程
func (b *Biz) PrepareSourceIndices(ctx context.Context, sourceId uuid.UUID) error {
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

	docConverter, ok := b.docConverters[source.Kind]
	if !ok {
		return errors.ErrParams.Msgf("can not convert source for kind %s", source.Kind)
	}

	result, err := docConverter.Handle(ctx, source)
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

	docs := make([]*vecschema.SourceDoc, len(result.Docs))
	for i, doc := range result.Docs {
		embedding := embeddings[i]
		embedding32 := make([]float32, len(embedding))
		for j, v := range embedding {
			embedding32[j] = float32(v)
		}
		docs[i] = &vecschema.SourceDoc{
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
	qids := slices.Unique(query.SourceIds)
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

type RetrieveSourceDocsQuery struct {
	NotebookId uuid.UUID
	Query      string
	SourceIds  []uuid.UUID
	Count      int
}

// 召回来源片段
func (b *Biz) RetrieveSourceDocs(
	ctx context.Context,
	query *RetrieveSourceDocsQuery,
) ([]*model.SourceDoc, error) {
	var (
		notebookId = query.NotebookId.String()
		sourceIds  = slices.Map(
			slices.Unique(query.SourceIds),
			func(id uuid.UUID) string { return id.String() },
		)
	)

	queryEmbeddings, err := b.embedder.EmbedStrings(ctx, []string{query.Query})
	if err != nil {
		return nil, errors.Wrapf(errors.ErrEmbed,
			"query embedding failed, query=%s, notebook_id=%s",
			query.Query, notebookId)
	}

	if len(queryEmbeddings) == 0 {
		return nil, errors.Wrapf(errors.ErrEmbed,
			"query embedding result is empty, query=%s, notebook_id=%s",
			query.Query, query.NotebookId)
	}

	queryEmbedding := float64ToFloat32(queryEmbeddings[0])
	docs, err := b.sourceDocStore.Query(ctx,
		&vecschema.SourceDocQueryParams{
			NotebookId: notebookId,
			SourceIds:  sourceIds,
			Embedding:  queryEmbedding,
			Target:     query.Query,
			Limit:      query.Count,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "query source docs failed")
	}
	if len(docs) == 0 {
		return []*model.SourceDoc{}, nil
	}

	queriedDocs := make([]*model.SourceDoc, 0, len(docs))
	for _, doc := range docs {
		queriedDoc, err := model.NewSourceDoc(doc)
		if err != nil {
			slog.ErrorContext(ctx,
				"new source doc failed",
				slog.Any("err", err),
				slog.String("doc_id", doc.Id))
			continue
		}
		queriedDocs = append(queriedDocs, queriedDoc)
	}

	return queriedDocs, nil
}
