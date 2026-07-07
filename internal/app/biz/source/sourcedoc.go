package source

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/indices"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"
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
	err := b.sourceDocStore.BatchDelete(ctx, &schema.SourceDocBatchDeleteParams{
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

type SimilaritySearchSourceDocsQuery struct {
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
		&schema.SourceDocGetParams{
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
		if err := b.PopulateSourceDocs(
			ctx,
			query.NotebookId,
			[]*model.SourceDoc{sourceDoc},
		); err != nil {
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
		&schema.SourceDocBatchGetParams{
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
		&schema.SourceDocListParams{
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
func (b *Biz) SimilaritySearchSourceDocs(
	ctx context.Context,
	query *SimilaritySearchSourceDocsQuery,
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
		&schema.SourceDocQueryParams{
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
		&schema.SourceDocListParams{
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

// 额外填充 SourceDoc 字段。
//
// 额外字段定义见 [model.SourceDoc]
func (s *Biz) PopulateSourceDocs(
	ctx context.Context,
	notebookId uuid.UUID,
	docs []*model.SourceDoc,
) error {
	if len(docs) == 0 {
		return nil
	}

	helper := newSourceDocPopulateHelper(s, notebookId.String())
	deriveMetas, treeMetas, posesBySource := helper.collectPopulateMetas(ctx, docs)
	if len(deriveMetas) == 0 && len(treeMetas) == 0 {
		return nil
	}

	docsBySourcePos, err := helper.loadDocsBySourcePos(ctx, posesBySource)
	if err != nil {
		return err
	}

	helper.populateDerivationIDs(ctx, deriveMetas, docsBySourcePos)
	helper.populateTreeMetaIDs(ctx, treeMetas, docsBySourcePos)

	return nil
}

type deriveMeta struct {
	doc      *model.SourceDoc
	sourceId string
	poses    []int32
}

type treeMetaResolve struct {
	doc         *model.SourceDoc
	sourceId    string
	parentPos   *int
	childrenPos []int
}

type sourceDocPopulateHelper struct {
	biz        *Biz
	notebookID string
}

func newSourceDocPopulateHelper(
	biz *Biz,
	notebookID string,
) *sourceDocPopulateHelper {
	return &sourceDocPopulateHelper{
		biz:        biz,
		notebookID: notebookID,
	}
}

func (h *sourceDocPopulateHelper) collectPopulateMetas(
	ctx context.Context,
	docs []*model.SourceDoc,
) ([]*deriveMeta, []*treeMetaResolve, map[string][]int32) {
	deriveMetas := make([]*deriveMeta, 0, len(docs))
	treeMetas := make([]*treeMetaResolve, 0, len(docs))
	posesBySource := make(map[string][]int32)

	for _, doc := range docs {
		if doc == nil {
			continue
		}
		sourceID := doc.SourceId.String()

		if dmeta, queryPoses := h.buildDeriveMeta(ctx, doc, sourceID); dmeta != nil {
			deriveMetas = append(deriveMetas, dmeta)
			posesBySource[sourceID] = append(posesBySource[sourceID], queryPoses...)
		}
		if tmeta, queryPoses := h.buildTreeMetaResolve(doc, sourceID); tmeta != nil {
			treeMetas = append(treeMetas, tmeta)
			posesBySource[sourceID] = append(posesBySource[sourceID], queryPoses...)
		}
	}

	return deriveMetas, treeMetas, posesBySource
}

func (h *sourceDocPopulateHelper) buildDeriveMeta(
	ctx context.Context,
	doc *model.SourceDoc,
	sourceID string,
) (*deriveMeta, []int32) {
	derivationPos := doc.DerivationPos()
	if derivationPos == "" {
		return nil, nil
	}

	bm, err := bitmap.NewFrom(derivationPos)
	if err != nil {
		slog.WarnContext(ctx, "decode source doc derivation pos failed",
			slog.String("doc_id", doc.Id),
			slog.String("source_id", sourceID),
			slog.String("notebook_id", h.notebookID),
			slog.Any("err", err),
		)
		return nil, nil
	}

	setPos := bm.GetAllSet()
	if len(setPos) == 0 {
		return nil, nil
	}
	chunkPoses := make([]int32, 0, len(setPos))
	for _, pos := range setPos {
		chunkPoses = append(chunkPoses, int32(pos))
	}
	if len(chunkPoses) == 0 {
		return nil, nil
	}

	return &deriveMeta{
		doc:      doc,
		sourceId: sourceID,
		poses:    chunkPoses,
	}, chunkPoses
}

func (h *sourceDocPopulateHelper) buildTreeMetaResolve(
	doc *model.SourceDoc,
	sourceID string,
) (*treeMetaResolve, []int32) {
	if doc == nil || doc.TreeMeta == nil {
		return nil, nil
	}

	parentPos, hasParent := doc.TreeMeta.ParentPos()
	childrenPos := doc.TreeMeta.ChildrenPos()
	if !hasParent && len(childrenPos) == 0 {
		return nil, nil
	}

	meta := &treeMetaResolve{
		doc:         doc,
		sourceId:    sourceID,
		childrenPos: childrenPos,
	}
	queryPoses := make([]int32, 0, len(childrenPos)+1)
	if hasParent {
		parentPosCopy := parentPos
		meta.parentPos = &parentPosCopy
		queryPoses = append(queryPoses, int32(parentPosCopy))
	}
	for _, childPos := range childrenPos {
		queryPoses = append(queryPoses, int32(childPos))
	}

	return meta, queryPoses
}

func (h *sourceDocPopulateHelper) loadDocsBySourcePos(
	ctx context.Context,
	posesBySource map[string][]int32,
) (map[string]map[int32]*schema.SourceDoc, error) {
	docsBySourcePos := make(map[string]map[int32]*schema.SourceDoc, len(posesBySource))
	var (
		mu sync.Mutex
		eg errgroup.Group
	)
	for sourceId, chunkPoses := range posesBySource {
		sourceID := sourceId
		posList := append([]int32(nil), slices.Unique(chunkPoses)...)
		eg.Go(func() error {
			sourceDocs, err := h.biz.sourceDocStore.ListByChunkPos(ctx,
				&schema.SourceDocListByChunkPosParams{
					NotebookId: h.notebookID,
					SourceId:   sourceID,
					ChunkPoses: posList,
				})
			if err != nil {
				return errors.WithMessagef(err,
					"list source docs by chunk pos failed, notebook_id=%s, source_id=%s",
					h.notebookID,
					sourceID,
				)
			}
			docsByPos := make(map[int32]*schema.SourceDoc, len(sourceDocs))
			for _, sourceDoc := range sourceDocs {
				docsByPos[sourceDoc.ChunkPos] = sourceDoc
			}
			mu.Lock()
			docsBySourcePos[sourceID] = docsByPos
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return docsBySourcePos, nil
}

func (h *sourceDocPopulateHelper) populateDerivationIDs(
	ctx context.Context,
	deriveMetas []*deriveMeta,
	docsBySourcePos map[string]map[int32]*schema.SourceDoc,
) {
	for _, meta := range deriveMetas {
		docsByPos := docsBySourcePos[meta.sourceId]
		if len(docsByPos) == 0 {
			continue
		}

		derivation := make([]uuid.UUID, 0, len(meta.poses))
		seen := make(map[uuid.UUID]struct{}, len(meta.poses))
		for _, pos := range meta.poses {
			derivedDoc, ok := docsByPos[pos]
			if !ok || derivedDoc == nil {
				continue
			}
			derivationID, err := uuid.ParseString(derivedDoc.Id)
			if err != nil {
				slog.WarnContext(ctx, "ignore invalid source doc id while populating derivation ids",
					slog.String("doc_id", derivedDoc.Id),
					slog.Int64("chunk_pos", int64(pos)),
					slog.String("source_id", meta.sourceId),
					slog.String("notebook_id", h.notebookID),
					slog.Any("err", err),
				)
				continue
			}
			if _, ok := seen[derivationID]; ok {
				continue
			}
			seen[derivationID] = struct{}{}
			derivation = append(derivation, derivationID)
		}
		meta.doc.Derivation = derivation
	}
}

func (h *sourceDocPopulateHelper) populateTreeMetaIDs(
	ctx context.Context,
	treeMetas []*treeMetaResolve,
	docsBySourcePos map[string]map[int32]*schema.SourceDoc,
) {
	for _, meta := range treeMetas {
		docsByPos := docsBySourcePos[meta.sourceId]
		if len(docsByPos) == 0 || meta.doc == nil || meta.doc.TreeMeta == nil {
			continue
		}
		h.populateParentID(ctx, meta, docsByPos)
		meta.doc.TreeMeta.Children = h.collectChildIDs(ctx, meta, docsByPos)
	}
}

func (h *sourceDocPopulateHelper) populateParentID(
	ctx context.Context,
	meta *treeMetaResolve,
	docsByPos map[int32]*schema.SourceDoc,
) {
	if meta == nil || meta.parentPos == nil {
		return
	}
	parentDoc, ok := docsByPos[int32(*meta.parentPos)]
	if !ok || parentDoc == nil {
		return
	}
	parentID, err := uuid.ParseString(parentDoc.Id)
	if err != nil {
		slog.WarnContext(ctx, "ignore invalid parent source doc id while populating tree meta",
			slog.String("doc_id", parentDoc.Id),
			slog.Int64("chunk_pos", int64(*meta.parentPos)),
			slog.String("source_id", meta.sourceId),
			slog.String("notebook_id", h.notebookID),
			slog.Any("err", err),
		)
		return
	}
	meta.doc.TreeMeta.ParentId = parentID
}

func (h *sourceDocPopulateHelper) collectChildIDs(
	ctx context.Context,
	meta *treeMetaResolve,
	docsByPos map[int32]*schema.SourceDoc,
) []uuid.UUID {
	if meta == nil {
		return nil
	}

	children := make([]uuid.UUID, 0, len(meta.childrenPos))
	seen := make(map[uuid.UUID]struct{}, len(meta.childrenPos))
	for _, childPos := range meta.childrenPos {
		childDoc, ok := docsByPos[int32(childPos)]
		if !ok || childDoc == nil {
			continue
		}
		childID, err := uuid.ParseString(childDoc.Id)
		if err != nil {
			slog.WarnContext(ctx, "ignore invalid child source doc id while populating tree meta",
				slog.String("doc_id", childDoc.Id),
				slog.Int64("chunk_pos", int64(childPos)),
				slog.String("source_id", meta.sourceId),
				slog.String("notebook_id", h.notebookID),
				slog.Any("err", err),
			)
			continue
		}
		if _, ok := seen[childID]; ok {
			continue
		}
		seen[childID] = struct{}{}
		children = append(children, childID)
	}

	return children
}
