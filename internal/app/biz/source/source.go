package source

import (
	"context"
	"log/slog"
	"net/url"
	"sync"
	"time"

	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	llm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/openai"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	vecschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einoembed "github.com/cloudwego/eino/components/embedding"
)

var (
	ErrSourceNotFound             = errors.ErrParams.Msg("source not found")
	ErrSourceContentTooLong       = errors.ErrParams.Msg("source content too long")
	ErrSourceCountExceedsMaxCount = errors.ErrParams.Msg("source count exceeds max count")
)

type Biz struct {
	objectStorage  storage.Storage
	sourceStore    dal.SourceStore
	sourceDocStore vectordb.SourceDocStore

	llmGateway    *openai.Gateway
	embedder      einoembed.Embedder
	sourceIndexer *SourceIndexer
}

func New(
	objectStorage storage.Storage,
	sourceStore dal.SourceStore,
	sourceDocStore vectordb.SourceDocStore,
	llmGateway *openai.Gateway,
	embeddingGateway *llm.EmbeddingGateway,
	prompt *bizprompt.Prompt,
) (*Biz, error) {
	providerType := conf.Global().Embedding.Type
	embedder, err := embeddingGateway.GetProvider(providerType)
	if err != nil {
		return nil, errors.WithMessage(err, "get embedder from gateway failed")
	}

	b := &Biz{
		objectStorage:  objectStorage,
		sourceStore:    sourceStore,
		sourceDocStore: sourceDocStore,
		embedder:       embedder,
		sourceIndexer:  NewSourceIndexer(embedder, sourceDocStore, objectStorage, llmGateway, prompt),
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

func (b *Biz) BatchGetSources(
	ctx context.Context,
	notebookId uuid.UUID, sourceIds []uuid.UUID,
) ([]*model.Source, error) {
	rows, err := b.sourceStore.ListByNotebookIdAndIds(
		ctx,
		notebookId,
		sourceIds,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "store list sources failed")
	}

	sources := make([]*model.Source, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, model.NewSourceFrom(row))
	}

	return sources, nil
}

func (b *Biz) GetDecodedSource(
	ctx context.Context,
	sourceId uuid.UUID,
	options ...SourceOption,
) (*model.DecodedSource, error) {
	opt := newSourceOption(options...)
	rawSource, err := b.GetSource(ctx, sourceId)
	if err != nil {
		return nil, err
	}

	decodedSource, err := model.NewDecodedSource(rawSource)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "new decoded source failed, source_id=%s", sourceId)
	}

	if opt.populateContentRef {
		err = decodedSource.PopulateContentRef(func(storeKey string) (string, error) {
			req := &storage.PresignedGetObjectRequest{
				Key: storeKey,
			}
			resp, err := b.objectStorage.PresignedGetObject(ctx, req)
			if err != nil {
				return "", err
			}

			return resp.Url, nil
		})
		if err != nil {
			slog.ErrorContext(ctx, "populate content ref failed",
				slog.Any("err", err),
				slog.String("source_id", sourceId.String()),
			)
		}
	}

	return decodedSource, nil
}

func (b *Biz) BatchGetDecodedSources(
	ctx context.Context,
	notebookId uuid.UUID,
	sourceIds []uuid.UUID,
	options ...SourceOption,
) ([]*model.DecodedSource, error) {
	opt := newSourceOption(options...)
	sources, err := b.BatchGetSources(ctx, notebookId, sourceIds)
	if err != nil {
		return nil, errors.WithMessage(err, "batch get sources failed")
	}

	decodedSources := make([]*model.DecodedSource, 0, len(sources))
	for _, source := range sources {
		decodedSource, err := model.NewDecodedSource(source)
		if err != nil {
			return nil, errors.WithMessagef(err, "new decoded source failed, source_id=%s", source.Id)
		}
		decodedSources = append(decodedSources, decodedSource)
	}

	if opt.populateContentRef {
		b.batchPopulateContentRef(ctx, decodedSources)
	}

	return decodedSources, nil
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
	options ...SourceOption,
) ([]*model.DecodedSource, error) {
	opt := newSourceOption(options...)
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

	decodedSources := make([]*model.DecodedSource, 0, len(sources))
	for _, source := range sources {
		sc, err := model.NewDecodedSource(source)
		if err != nil {
			slog.ErrorContext(ctx, "new decoded source failed",
				slog.Any("err", err),
				slog.String("source_id", source.Id.String()),
			)
			return nil, errors.WithMessagef(err, "new source with content failed, source_id=%s", source.Id)
		}

		decodedSources = append(decodedSources, sc)
	}

	if opt.populateContentRef {
		b.batchPopulateContentRef(ctx, decodedSources)
	}

	return decodedSources, nil
}

func (b *Biz) batchPopulateContentRef(
	ctx context.Context,
	decodedSources []*model.DecodedSource,
) {
	// 文件来源的预签名地址并发拉取
	var wg sync.WaitGroup
	for i := range decodedSources {
		sc := decodedSources[i]
		if !sc.IsContentRef() {
			continue
		}

		wg.Add(1)
		safe.Go(ctx, func() {
			defer wg.Done()

			err := sc.PopulateContentRef(func(storeKey string) (string, error) {
				req := &storage.PresignedGetObjectRequest{
					Key:         sc.ContentFile.StoreKey,
					Inline:      true,
					ContentType: previewResponseContentType(sc.ContentFile.Format),
				}
				resp, err := b.objectStorage.PresignedGetObject(ctx, req)
				if err != nil {
					return "", err
				}

				return resp.Url, nil
			})
			if err != nil {
				// 对单个来源失败仅记录日志，不影响整体列表返回。
				slog.ErrorContext(ctx, "get file source object url failed",
					slog.String("source_id", sc.Id.String()),
					slog.Any("err", err),
				)
			}
		})
	}
	wg.Wait()
}

// 获取notebook的全部来源
func (b *Biz) FetchNotebookSources(
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

func (b *Biz) FetchNotebookDecodedSources(
	ctx context.Context,
	notebookId uuid.UUID,
	options ...SourceOption,
) ([]*model.DecodedSource, error) {
	opt := newSourceOption(options...)
	var (
		limit             = 100
		offset            = 0
		allDecodedSources = make([]*model.DecodedSource, 0, limit)
	)

	for {
		sources, err := b.ListDecodedSourcesByNotebook(ctx,
			&ListDecodedSourcesByNotebookQuery{
				NotebookId: notebookId,
				Limit:      limit,
				Offset:     offset,
			})
		if err != nil {
			return nil, errors.WithMessagef(err,
				"list notebook decoded sources failed, notebook_id=%s",
				notebookId)
		}
		if len(sources) == 0 {
			break
		}

		allDecodedSources = append(allDecodedSources, sources...)
		if len(sources) < limit {
			break
		}
		offset += limit
	}

	if opt.populateContentRef {
		b.batchPopulateContentRef(ctx, allDecodedSources)
	}

	return allDecodedSources, nil
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

func (b *Biz) DeleteSource(ctx context.Context, sourceId uuid.UUID) error {
	source, err := b.GetDecodedSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, ErrSourceNotFound) || errors.Is(err, errors.ErrNoRecord) {
			return ErrSourceNotFound
		}

		return errors.WithMessagef(err, "get source failed before deleting, id=%s", sourceId)
	}

	err = b.sourceStore.BatchDelete(ctx, []uuid.UUID{sourceId})
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

	storeKeys, _ := source.ContentRefKeys()
	if len(storeKeys) > 0 {
		storeKeys = slices.Unique(storeKeys)
		if len(storeKeys) > 0 {
			err = b.objectStorage.BatchDeleteObject(ctx, &storage.BatchDeleteObjectRequest{
				Keys: storeKeys,
			})
			if err != nil {
				slog.WarnContext(ctx, "delete source object failure",
					slog.String("source_id", sourceId.String()),
					slog.Int("key_count", len(storeKeys)),
					slog.Any("err", err),
				)
			}
		}
	}

	return nil
}

func (b *Biz) DeleteNotebookSources(
	ctx context.Context,
	notebookId uuid.UUID,
) error {
	sources, err := b.FetchNotebookSources(ctx, notebookId)
	if err != nil {
		return errors.WithMessagef(err, "get notebook sources failed, notebook_id=%s", notebookId)
	}

	targets := b.buildDeleteTargets(ctx, sources)

	err = b.deleteRowsAndDocs(ctx, notebookId, targets.sourceIDs)
	if err != nil {
		return err
	}

	b.deleteObjects(ctx, notebookId, targets.objectStoreKeys)

	// 兜底清理：确保未出现在分页结果中的来源记录也被删除（例如 inited 状态）。
	err = b.sourceStore.DeleteByNotebookId(ctx, notebookId)
	if err != nil {
		return errors.WithMessagef(err, "delete sources by notebook failed, notebook_id=%s", notebookId)
	}

	return nil
}

// deleteTargets 汇总一次删除所需的记录与对象键。
type deleteTargets struct {
	sourceIDs       []uuid.UUID
	objectStoreKeys []string
}

// buildDeleteTargets 单次遍历收集 source ids、doc ids 与对象键。
func (b *Biz) buildDeleteTargets(
	ctx context.Context,
	sources []*model.Source,
) *deleteTargets {
	targets := &deleteTargets{
		sourceIDs:       make([]uuid.UUID, 0, len(sources)),
		objectStoreKeys: make([]string, 0, len(sources)*2),
	}

	for _, source := range sources {
		if source == nil {
			continue
		}
		keys, err := source.ContentRefKeys()
		if err != nil {
			slog.WarnContext(ctx, "get source content keys failed",
				slog.String("source_id", source.Id.String()),
				slog.Any("err", err),
			)
		}

		targets.sourceIDs = append(targets.sourceIDs, source.Id)
		if len(keys) > 0 {
			targets.objectStoreKeys = append(targets.objectStoreKeys, keys...)
		}
	}

	targets.objectStoreKeys = slices.Unique(targets.objectStoreKeys)
	return targets
}

// deleteRowsAndDocs 是强一致删除路径：记录与向量索引必须删除成功。
func (b *Biz) deleteRowsAndDocs(
	ctx context.Context,
	notebookId uuid.UUID,
	sourceIDs []uuid.UUID,
) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	sourceIDStrs := make([]string, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		sourceIDStrs = append(sourceIDStrs, sourceID.String())
	}

	err := b.sourceStore.BatchDelete(ctx, sourceIDs)
	if err != nil {
		return errors.WithMessagef(err, "batch delete sources failed, notebook_id=%s", notebookId)
	}

	err = b.sourceDocStore.BatchDelete(ctx, &vecschema.SourceDocBatchDeleteParams{
		NotebookId: notebookId.String(),
		SourceId:   sourceIDStrs,
	})
	if err != nil {
		return errors.WithMessagef(err, "delete source docs by notebook failed, notebook_id=%s", notebookId)
	}

	return nil
}

// deleteObjects 是 best-effort 清理：失败仅记录日志，不中断主流程。
func (b *Biz) deleteObjects(
	ctx context.Context,
	notebookId uuid.UUID,
	storeKeys []string,
) {
	if len(storeKeys) == 0 {
		return
	}

	err := b.objectStorage.BatchDeleteObject(ctx, &storage.BatchDeleteObjectRequest{
		Keys: storeKeys,
	})
	if err != nil {
		slog.WarnContext(ctx, "batch delete source objects failure",
			slog.String("notebook_id", notebookId.String()),
			slog.Int("key_count", len(storeKeys)),
			slog.Any("err", err),
		)
	}
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
	Id  uuid.UUID
	Key string
}

func (b *Biz) UpdateParsedContent(ctx context.Context, cmd *UpdateParsedContentCommand) error {
	err := b.sourceStore.UpdateParsedContent(ctx, &schema.SourceUpdateParsedContentParams{
		Id:               cmd.Id,
		ParsedContentKey: cmd.Key,
		UpdatedAt:        time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessagef(err, "store update source parsed content failed, source_id=%s", cmd.Id)
	}

	return nil
}

type UploadParsedContentCommand struct {
	SourceId          uuid.UUID
	NotebookId        uuid.UUID
	ParsedContent     []byte
	ParsedContentType string
}

func (b *Biz) UploadParsedContent(ctx context.Context, cmd *UploadParsedContentCommand) error {
	if len(cmd.ParsedContent) == 0 {
		return nil
	}

	storeKey := formatParsedContentStoreKey(cmd.SourceId, cmd.NotebookId)
	err := b.objectStorage.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         storeKey,
		Body:        cmd.ParsedContent,
		ContentType: cmd.ParsedContentType,
	})
	if err != nil {
		return errors.WithMessagef(err, "upload source parsed content failed, source_id=%s", cmd.SourceId)
	}

	err = b.UpdateParsedContent(ctx, &UpdateParsedContentCommand{
		Id:  cmd.SourceId,
		Key: storeKey,
	})
	if err != nil {
		return errors.WithMessagef(err,
			"update source parsed content failed after upload, source_id=%s",
			cmd.SourceId,
		)
	}

	return nil
}

func (b *Biz) DeleteParsedContent(ctx context.Context, source *model.DecodedSource) error {
	if source.ParsedContentKey != "" {
		err := b.objectStorage.BatchDeleteObject(ctx, &storage.BatchDeleteObjectRequest{
			Keys: []string{source.ParsedContentKey},
		})
		if err != nil {
			return errors.WithMessagef(err, "delete source parsed content failed, source_id=%s", source.Id)
		}
	}

	err := b.UpdateParsedContent(ctx, &UpdateParsedContentCommand{
		Id:  source.Id,
		Key: "",
	})
	if err != nil {
		return errors.WithMessagef(err,
			"clear source parsed content metadata failed, source_id=%s",
			source.Id,
		)
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

type CheckSourceIdsReadyQuery struct {
	NotebookId uuid.UUID
	SourceIds  []uuid.UUID
}

// 检查source ids是否ready且属于notebookid
func (b *Biz) CheckSourceIdsReady(
	ctx context.Context,
	query *CheckSourceIdsReadyQuery,
) ([]uuid.UUID, error) {
	sourceIds := slices.Unique(query.SourceIds)
	rows, err := b.sourceStore.ListByNotebookIdAndIds(
		ctx,
		query.NotebookId,
		sourceIds,
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

func (b *Biz) GetSourceUser(ctx context.Context, sourceId uuid.UUID) (string, error) {
	source, err := b.GetSource(ctx, sourceId)
	if err != nil {
		return "", errors.WithMessagef(err, "get source user failed, source_id=%s", sourceId)
	}

	return source.OwnerId, nil
}

func (b *Biz) BatchPopulateFullSources(
	ctx context.Context,
	sources []*model.FullSource,
	options ...SourceOption,
) error {
	opt := newSourceOption(options...)

	var wg sync.WaitGroup
	for _, source := range sources {
		wg.Go(func() {
			req := &storage.PresignedGetObjectRequest{
				Key: source.ParsedContentKey,
			}
			if opt.forDownload {
				req.Attachment = true
				req.AttachmentFilename = source.Title + ".md"
			}

			resp, err := b.objectStorage.PresignedGetObject(ctx, req)
			if err != nil {
				slog.ErrorContext(ctx, "get parsed content failed",
					slog.Any("err", err),
					slog.String("source_id", source.Id.String()),
				)
				return
			}
			source.ParsedContentUrl = resp.Url
		})
	}

	wg.Wait()

	return nil
}
