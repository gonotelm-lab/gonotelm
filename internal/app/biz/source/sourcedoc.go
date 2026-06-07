package source
import (
	"context"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/indices"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	vecschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/bitmap"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"golang.org/x/sync/errgroup"
)

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

	// 是否填充额外字段
	Populate bool
}

func (b *Biz) GetSourceDoc(
	ctx context.Context,
	query *GetSourceDocQuery,
) (*model.SourceDoc, error) {
	doc, err := b.sourceDocStore.Get(ctx,
		&vecschema.SourceDocGetParams{
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
	if query.Populate {
		if err := b.PopulateSourceDocs(ctx, query.NotebookId, []*model.SourceDoc{sourceDoc}); err != nil {
			slog.WarnContext(ctx, "populate source doc failed",
				slog.Any("err", err),
				slog.String("notebook_id", query.NotebookId.String()),
			)
		}
	}

	return sourceDoc, nil
}

type BatchGetSourceDocsQuery struct {
	NotebookId uuid.UUID
	SourceId   uuid.UUID
	DocIds     []string

	// 是否填充额外字段
	Populate bool
}

func (b *Biz) BatchGetSourceDocs(
	ctx context.Context,
	query *BatchGetSourceDocsQuery,
) ([]*model.SourceDoc, error) {
	docs, err := b.sourceDocStore.BatchGet(ctx,
		&vecschema.SourceDocBatchGetParams{
			NotebookId: query.NotebookId.String(),
			SourceId:   query.SourceId.String(),
			DocIds:     query.DocIds,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "batch get source docs failed")
	}

	sourceDocs := make([]*model.SourceDoc, 0, len(docs))
	for _, doc := range docs {
		sourceDoc, err := model.NewSourceDoc(doc)
		if err != nil {
			return nil, errors.WithMessagef(err, "new source doc failed, doc_id=%s", doc.Id)
		}
		sourceDocs = append(sourceDocs, sourceDoc)
	}

	if query.Populate {
		if err := b.PopulateSourceDocs(ctx, query.NotebookId, sourceDocs); err != nil {
			slog.WarnContext(ctx, "populate source docs failed",
				slog.Any("err", err),
				slog.String("notebook_id", query.NotebookId.String()),
			)
		}
	}

	return sourceDocs, nil
}

type ListSourceDocsQuery struct {
	NotebookId uuid.UUID
	SourceId   uuid.UUID

	// 是否填充额外字段
	Populate bool
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
	if query.Populate {
		if err := b.PopulateSourceDocs(ctx, query.NotebookId, sourceDocs); err != nil {
			slog.WarnContext(ctx, "populate source docs failed",
				slog.Any("err", err),
				slog.String("notebook_id", query.NotebookId.String()),
			)
		}
	}

	return sourceDocs, nil
}

// 召回来源片段
//
// 需要注意的是 来源片段中可能会存在派生的片段, 这些派生片段一般为一些总结性的语句片段
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
			"query embedding failed, query=%s, notebook_id=%s, err=%v",
			query.Query, notebookId, err)
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
	if err := b.PopulateSourceDocs(ctx, query.NotebookId, queriedDocs); err != nil {
		slog.WarnContext(ctx,
			"populate source docs failed",
			slog.Any("err", err),
			slog.String("notebook_id", notebookId),
		)
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

	return recoverDocTree(ctx, docs)
}

// 额外填充SourceDoc字段
func (s *Biz) PopulateSourceDocs(
	ctx context.Context,
	notebookId uuid.UUID,
	docs []*model.SourceDoc,
) error {
	if len(docs) == 0 {
		return nil
	}

	notebookID := notebookId.String()
	type deriveMeta struct {
		doc      *model.SourceDoc
		sourceId string
		poses    []int32
	}

	metas := make([]*deriveMeta, 0, len(docs))
	posesBySource := make(map[string][]int32)
	for _, doc := range docs {
		if doc == nil {
			continue
		}

		derivingPos := doc.DerivingPos()
		if derivingPos == "" {
			continue
		}

		bm, err := bitmap.NewFrom(derivingPos)
		if err != nil {
			slog.WarnContext(ctx, "decode source doc deriving pos failed",
				slog.String("doc_id", doc.Id),
				slog.String("source_id", doc.SourceId.String()),
				slog.String("notebook_id", notebookID),
				slog.Any("err", err),
			)
			continue
		}

		setPos := bm.GetAllSet()
		if len(setPos) == 0 {
			continue
		}

		chunkPoses := make([]int32, 0, len(setPos))
		for _, pos := range setPos {
			chunkPoses = append(chunkPoses, int32(pos))
		}
		if len(chunkPoses) == 0 {
			continue
		}

		sourceId := doc.SourceId.String()
		metas = append(metas, &deriveMeta{
			doc:      doc,
			sourceId: sourceId,
			poses:    chunkPoses,
		},
		)
		posesBySource[sourceId] = append(posesBySource[sourceId], chunkPoses...)
	}
	if len(metas) == 0 {
		return nil
	}

	docsBySourcePos := make(
		map[string]map[int32]*vecschema.SourceDoc,
		len(posesBySource),
	)
	var (
		mu sync.Mutex
		eg errgroup.Group
	)
	for sourceId, chunkPoses := range posesBySource {
		posList := slices.Unique(chunkPoses)
		eg.Go(func() error {
			sourceDocs, err := s.sourceDocStore.ListByChunkPos(ctx,
				&vecschema.SourceDocListByChunkPosParams{
					NotebookId: notebookID,
					SourceId:   sourceId,
					ChunkPoses: posList,
				})
			if err != nil {
				return errors.WithMessagef(err,
					"list source docs by chunk pos failed, notebook_id=%s, source_id=%s",
					notebookID,
					sourceId,
				)
			}

			docsByPos := make(map[int32]*vecschema.SourceDoc, len(sourceDocs))
			for _, sourceDoc := range sourceDocs {
				docsByPos[sourceDoc.ChunkPos] = sourceDoc
			}
			mu.Lock()
			docsBySourcePos[sourceId] = docsByPos
			mu.Unlock()

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	for _, meta := range metas {
		docsByPos := docsBySourcePos[meta.sourceId]
		if len(docsByPos) == 0 {
			continue
		}

		derivedFrom := make([]uuid.UUID, 0, len(meta.poses))
		seen := make(map[uuid.UUID]struct{}, len(meta.poses))
		for _, pos := range meta.poses {
			derivedDoc, ok := docsByPos[pos]
			if !ok || derivedDoc == nil {
				continue
			}

			derivedFromId, err := uuid.ParseString(derivedDoc.Id)
			if err != nil {
				slog.WarnContext(ctx, "ignore invalid source doc id while populating deriving ids",
					slog.String("doc_id", derivedDoc.Id),
					slog.Int64("chunk_pos", int64(pos)),
					slog.String("source_id", meta.sourceId),
					slog.String("notebook_id", notebookID),
					slog.Any("err", err),
				)
				continue
			}
			if _, ok := seen[derivedFromId]; ok {
				continue
			}
			seen[derivedFromId] = struct{}{}
			derivedFrom = append(derivedFrom, derivedFromId)
		}
		meta.doc.DerivedFrom = derivedFrom
	}

	return nil
}
