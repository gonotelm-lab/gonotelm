package source

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/indices"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	vecschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	einoembed "github.com/cloudwego/eino/components/embedding"
)

var (
	ErrSourceNotFound             = errors.New("source not found")
	ErrSourceContentTooLong       = errors.New("source content too long")
	ErrSourceCountExceedsMaxCount = errors.New("source count exceeds max count")
)

type Biz struct {
	objectStorage  storage.Storage
	sourceStore    dal.SourceStore
	sourceDocStore vectordal.SourceDocStore

	llmGateway    *gateway.Gateway
	embedder      einoembed.Embedder
	sourceIndexer *SourceIndexer
}

func New(
	objectStorage storage.Storage,
	sourceStore dal.SourceStore,
	sourceDocStore vectordal.SourceDocStore,
	llmGateway *gateway.Gateway,
) (*Biz, error) {
	embedder, err := embedding.New(
		context.Background(),
		&conf.Global().Embedding,
		embedding.NewRedisCacher(cache.GetRedis()),
	)
	if err != nil {
		return nil, errors.WithMessage(err, "new embedder failed")
	}

	b := &Biz{
		objectStorage:  objectStorage,
		sourceStore:    sourceStore,
		sourceDocStore: sourceDocStore,
		embedder:       embedder,
		sourceIndexer:  NewSourceIndexer(embedder, sourceDocStore, objectStorage, llmGateway),
		llmGateway:     llmGateway,
	}

	return b, nil
}

func (b *Biz) GetSource(ctx context.Context, sourceId uuid.UUID) (*model.Source, error) {
	source, err := b.sourceStore.GetById(ctx, sourceId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, ErrSourceNotFound
		}
		return nil, errors.WithMessage(err, "store get source failed")
	}

	return model.NewSourceFrom(source), nil
}

func (b *Biz) GetDecodedSource(ctx context.Context, sourceId uuid.UUID) (*model.DecodedSource, error) {
	rawSource, err := b.GetSource(ctx, sourceId)
	if err != nil {
		return nil, err
	}

	decodedSource, err := model.NewDecodedSource(rawSource)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "new decoded source failed, source_id=%s", sourceId)
	}

	return decodedSource, nil
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

type ListDecodedSourcesByNotebookQuery struct {
	NotebookId uuid.UUID
	Limit      int
	Offset     int
}

func (b *Biz) ListDecodedSourcesByNotebook(
	ctx context.Context, query *ListDecodedSourcesByNotebookQuery,
) ([]*model.DecodedSource, error) {
	rows, err := b.sourceStore.ListByNotebookId(
		ctx,
		query.NotebookId,
		query.Limit,
		query.Offset,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "store list sources failed")
	}

	sources := make([]*model.Source, 0, len(rows))
	for i := range rows {
		row := rows[i]
		sources = append(sources, model.NewSourceFrom(row))
	}

	sourcesWithContents := make([]*model.DecodedSource, 0, len(sources))
	for _, source := range sources {
		sc, err := model.NewDecodedSource(source)
		if err != nil {
			slog.ErrorContext(ctx, "new decoded source failed",
				slog.Any("err", err),
				slog.String("source_id", source.Id.String()),
			)
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

// 获取notebook的全部来源
func (b *Biz) GetAllNotebookSources(
	ctx context.Context,
	notebookId uuid.UUID,
) ([]*model.Source, error) {
	var (
		limit      = 100
		offset     = 0
		allSources = make([]*model.Source, 0, limit)
	)

	for {
		sources, err := b.sourceStore.ListByNotebookId(ctx, notebookId, limit, offset)
		if err != nil {
			return nil, errors.WithMessagef(err, "list notebook sources failed, notebook_id=%s", notebookId)
		}
		if len(sources) == 0 {
			break
		}
		for _, source := range sources {
			allSources = append(allSources, model.NewSourceFrom(source))
		}
		offset += limit
	}

	return allSources, nil
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

func (b *Biz) UpdateStatus(ctx context.Context, sourceId uuid.UUID, status model.SourceStatus) error {
	err := b.sourceStore.UpdateStatus(ctx, &schema.SourceUpdateStatusParams{
		Id:        sourceId,
		Status:    status.String(),
		UpdatedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessagef(err, "store update source status failed, id=%s", sourceId)
	}

	return nil
}

func (b *Biz) DeleteSource(ctx context.Context, sourceId uuid.UUID) (*model.DecodedSource, error) {
	source, err := b.GetDecodedSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, ErrSourceNotFound) || errors.Is(err, errors.ErrNoRecord) {
			return nil, ErrSourceNotFound
		}
		return nil, errors.WithMessagef(err, "get source failed before deleting, id=%s", sourceId)
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
		return nil, errors.WithMessagef(err, "store delete source failed, id=%s", sourceId)
	}

	err = b.sourceDocStore.BatchDelete(ctx, &vecschema.SourceDocBatchDeleteParams{
		NotebookId: source.NotebookId.String(),
		SourceId:   []string{source.Id.String()},
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "delete source docs failed, source_id=%s", sourceId)
	}

	if fileStoreKey != "" {
		err = b.objectStorage.DeleteObject(ctx, &storage.DeleteObjectRequest{
			Key: fileStoreKey,
		})
		if err != nil {
			slog.WarnContext(ctx, "delete source object failure",
				slog.String("source_id", sourceId.String()),
				slog.String("key", fileStoreKey),
				slog.Any("err", err),
			)
		}
	}

	return source, nil
}

type UpdateContentCommand struct {
	Id      uuid.UUID
	Content []byte
	Status  model.SourceStatus
	Title   string
}

func (b *Biz) UpdateContent(ctx context.Context, cmd *UpdateContentCommand) error {
	err := b.sourceStore.Update(ctx, &schema.SourceUpdateParams{
		Id:        cmd.Id,
		Content:   cmd.Content,
		Status:    cmd.Status.String(),
		Title:     cmd.Title,
		UpdatedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessagef(err, "store update source content failed, id=%s", cmd.Id)
	}

	return nil
}

func (b *Biz) UpdateTitle(ctx context.Context, sourceId uuid.UUID, title string) error {
	err := b.sourceStore.UpdateTitle(ctx, &schema.SourceUpdateTitleParams{
		Id:        sourceId,
		Title:     title,
		UpdatedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessagef(err, "store update source title failed, id=%s", sourceId)
	}

	return nil
}

type UpdateParsedContentCommand struct {
	Id     uuid.UUID
	Parsed *model.ParsedSourceContent
}

func (b *Biz) UpdateParsedContent(ctx context.Context, cmd *UpdateParsedContentCommand) error {
	parsedContent, err := sonic.Marshal(cmd.Parsed)
	if err != nil {
		return errors.WithMessage(err, "marshal parsed content failed")
	}

	err = b.sourceStore.UpdateParsedContent(ctx, &schema.SourceUpdateParsedContentParams{
		Id:            cmd.Id,
		ParsedContent: parsedContent,
		UpdatedAt:     time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessagef(err, "store update source parsed content failed, source_id=%s", cmd.Id)
	}

	return nil
}

func (b *Biz) UpdateAbstract(ctx context.Context, sourceId uuid.UUID, abstract string) error {
	err := b.sourceStore.UpdateAbstract(ctx, &schema.SourceUpdateAbstractParams{
		Id:        sourceId,
		Abstract:  abstract,
		UpdatedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessagef(err, "store update source abstract failed, id=%s", sourceId)
	}

	return nil
}

type PrepareSourceIndicesResult struct {
	ParsedContent     []byte
	ParsedContentType string
	Chunks            []string
}

// 准备数据源
// 包含chunk + embedding的索引过程
func (b *Biz) PrepareSourceIndices(
	ctx context.Context,
	sourceId uuid.UUID,
) (*PrepareSourceIndicesResult, error) {
	source, err := b.GetSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, ErrSourceNotFound) {
			return &PrepareSourceIndicesResult{}, nil
		}
		return nil, errors.WithMessagef(err, "get source failed, id=%s", sourceId)
	}

	result, err := b.sourceIndexer.Prepare(ctx, source)
	if err != nil {
		return nil, errors.WithMessagef(err, "prepare source indices failed, source_id=%s", sourceId)
	}

	return result, nil
}

// 清理已存在的所有来源的索引
func (b *Biz) ClearSourceIndices(
	ctx context.Context,
	notebookId uuid.UUID,
	sourceId uuid.UUID,
) error {
	err := b.sourceDocStore.BatchDelete(ctx, &vecschema.SourceDocBatchDeleteParams{
		NotebookId: notebookId.String(),
		SourceId:   []string{sourceId.String()},
	})
	if err != nil {
		return errors.WithMessagef(err,
			"delete source docs failed, notebook_id=%s, source_id=%s",
			notebookId, sourceId)
	}

	return nil
}

type CheckSourceIdsReadyQuery struct {
	NotebookId uuid.UUID
	SourceIds  []uuid.UUID
}

// 检查source ids是否ready且属于notebookid
func (b *Biz) CheckSourceIdsReady(
	ctx context.Context,
	query *CheckSourceIdsReadyQuery,
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
		// 只要ready的source ids
		if row.Status != model.SourceStatusReady.String() {
			continue
		}

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

type GetSourceDocQuery struct {
	NotebookId uuid.UUID
	SourceId   uuid.UUID
	DocId      string
}

func (b *Biz) GetSourceDoc(
	ctx context.Context,
	query *GetSourceDocQuery,
) (*model.SourceDoc, error) {
	doc, err := b.sourceDocStore.Get(ctx, &vecschema.SourceDocGetParams{
		NotebookId: query.NotebookId.String(),
		SourceId:   query.SourceId.String(),
		DocId:      query.DocId,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "get source doc failed")
	}

	sourceDoc, err := model.NewSourceDoc(doc)
	if err != nil {
		return nil, errors.WithMessage(err, "new source doc failed")
	}

	return sourceDoc, nil
}

type ListSourceDocsQuery struct {
	NotebookId uuid.UUID
	SourceId   uuid.UUID
}

// 列出source的全部doc
func (b *Biz) ListSourceDocs(
	ctx context.Context,
	query *ListSourceDocsQuery,
) ([]*model.SourceDoc, error) {
	docs, err := b.sourceDocStore.List(ctx,
		&vecschema.SourceDocListParams{
			NotebookId: query.NotebookId.String(),
			SourceId:   query.SourceId.String(),
		})
	if err != nil {
		return nil, errors.WithMessage(err, "store list source docs failed")
	}

	sourceDocs := make([]*model.SourceDoc, 0, len(docs))
	for _, doc := range docs {
		sourceDoc, err := model.NewSourceDoc(doc)
		if err != nil {
			slog.ErrorContext(ctx, "new source doc failed",
				slog.Any("err", err),
				slog.String("doc_id", doc.Id),
				slog.String("source_id", query.SourceId.String()),
				slog.String("notebook_id", query.NotebookId.String()),
			)
			continue
		}
		sourceDocs = append(sourceDocs, sourceDoc)
	}

	return sourceDocs, nil
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

	queryEmbedding := slices.CastFloat[float64, float32](queryEmbeddings[0])
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

func (b *Biz) GetSourceDocTree(
	ctx context.Context,
	notebookId uuid.UUID,
	sourceId uuid.UUID,
) (*indices.DocTree, error) {
	var (
		sourceIdStr   = sourceId.String()
		notebookIdStr = notebookId.String()
	)

	docs, err := b.sourceDocStore.List(
		ctx,
		&vecschema.SourceDocListParams{
			NotebookId: notebookIdStr,
			SourceId:   sourceIdStr,
			BatchSize:  500,
		},
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "list source docs failed, source_id=%s", sourceIdStr)
	}

	return RecoverDocTree(ctx, docs)
}
