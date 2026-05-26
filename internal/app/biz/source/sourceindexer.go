package source

import (
	"context"
	"log/slog"
	stdslices "slices"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc"
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/indices"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	vecschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	"github.com/gonotelm-lab/gonotelm/pkg/bitmap"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/token"

	einoembed "github.com/cloudwego/eino/components/embedding"
)

const (
	sourceDocMetaDerivingPos = "_doc_deriving_pos"      // 派生节点的来源叶子节点pos bitmap
	sourceDocMetaLevel       = "_doc_tree_level"        // 派生节点在树中的层级
	sourceDocMetaChildrenPos = "_doc_node_children_pos" // 派生节点的子节点pos列表
)

// 构建来源的索引
type SourceIndexer struct {
	embedder            einoembed.Embedder
	embedBatchSize      int
	embedMaxConcurrency int
	sourceDocStore      vectordal.SourceDocStore

	docConverters  map[model.SourceKind]convertdoc.Handler
	docTreeBuilder *indices.DocTreeBuilder
}

func NewSourceIndexer(
	embedder einoembed.Embedder,
	sourceDocStore vectordal.SourceDocStore,
	objectStorage storage.Storage,
	llmGateway *gateway.Gateway,
) *SourceIndexer {
	hc := convertdoc.HandlerConfig{
		ChunkSize:   conf.Global().Chunking.Size,
		OverlapSize: conf.Global().Chunking.OverlapSize,
	}
	if hc.OverlapSize == 0 || hc.OverlapSize > hc.ChunkSize {
		hc.OverlapSize = int(float64(hc.ChunkSize) * 0.15)
	}

	return &SourceIndexer{
		embedder:            embedder,
		embedBatchSize:      conf.Global().Embedding.BatchSize,
		embedMaxConcurrency: conf.Global().Embedding.MaxConcurrency,
		sourceDocStore:      sourceDocStore,
		docConverters: map[model.SourceKind]convertdoc.Handler{
			model.SourceKindText: convertdoc.NewTextHandler(hc),
			model.SourceKindUrl:  convertdoc.NewUrlHandler(hc),
			model.SourceKindFile: convertdoc.NewFileObjectHandler(hc, objectStorage),
		},
		docTreeBuilder: indices.NewDocTreeBuilder(
			embedder,
			llmGateway,
			func(_ context.Context) string { return string(conf.Global().Logic.Source.ModelProvider) },
			func(_ context.Context) string { return conf.Global().Logic.Source.Model }),
	}
}

func (b *SourceIndexer) Prepare(
	ctx context.Context,
	source *model.Source,
) (*PrepareSourceIndicesResult, error) {
	slog.DebugContext(ctx, "prepare source indices, converting...", slog.String("source_id", source.Id.String()))
	result, err := b.convertSource(ctx, source)
	if err != nil {
		return nil, err
	}

	// 超过token限制的直接报错不处理
	estimatedToken := token.EstimateToken(pkgstring.FromBytes(result.ParsedContent))
	if estimatedToken > constants.MaxSourceTextContentToken {
		return nil, errors.Wrapf(ErrSourceContentTooLong,
			"source content too long, token count=%d, source_id=%s",
			estimatedToken,
			source.Id.String(),
		)
	}

	slog.DebugContext(ctx, "prepare source indices",
		slog.String("source_id", source.Id.String()),
		slog.Int("estimated_token", estimatedToken),
	)

	if result.ParsedContentType == model.MimeTypeMarkdown {
		// 对于有明显层级结构的markdown文档尝试使用语法树解析
	}

	textChunks, vsDocs, err := b.embedChunks(ctx, source, result)
	if err != nil {
		return nil, err
	}

	err = b.insertSourceDocs(ctx, vsDocs)
	if err != nil {
		return nil, errors.WithMessagef(err, "insert source docs failed")
	}

	return &PrepareSourceIndicesResult{
		ParsedContent:     result.ParsedContent,
		ParsedContentType: result.ParsedContentType,
		Chunks:            textChunks,
	}, nil
}

func (b *SourceIndexer) convertSource(
	ctx context.Context,
	source *model.Source,
) (*convertdoc.HandleResult, error) {
	docConverter, ok := b.docConverters[source.Kind]
	if !ok {
		return nil, errors.ErrParams.Msgf("can not convert source for kind %s", source.Kind)
	}

	result, err := docConverter.Handle(ctx, source)
	if err != nil {
		return nil, errors.WithMessagef(err, "embed source failed")
	}

	return result, nil
}

func (b *SourceIndexer) embedChunks(
	ctx context.Context,
	source *model.Source,
	result *convertdoc.HandleResult,
) ([]string, []*vecschema.SourceDoc, error) {
	texts := make([]string, 0, len(result.Docs))
	for _, doc := range result.Docs {
		texts = append(texts, doc.Content)
	}

	slog.DebugContext(ctx, "embedding source docs",
		slog.Int("text_size", len(texts)),
		slog.Int("batch_size", b.embedBatchSize),
		slog.Int("max_concurrency", b.embedMaxConcurrency),
		slog.String("source_id", source.Id.String()))

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
		return nil, nil, errors.WithMessagef(err, "embed docs failed")
	}
	if len(embeddings) != len(texts) {
		return nil, nil, errors.Wrapf(
			errors.ErrSerde,
			"embed result count mismatch, expected=%d, actual=%d",
			len(texts),
			len(embeddings),
		)
	}

	notebookIdStr := source.NotebookId.String()
	sourceIdStr := source.Id.String()
	docsLen := len(result.Docs)
	vsDocs := make([]*vecschema.SourceDoc, 0, docsLen)
	fallbackVsDocs := make([]*vecschema.SourceDoc, 0, docsLen)
	leafNodes := make([]*indices.DocTreeNode, 0, docsLen)
	idPosMapping := make(map[string]int, docsLen)
	for pos, doc := range result.Docs {
		vdoc := &vecschema.SourceDoc{
			Id:         doc.ID,
			NotebookId: notebookIdStr,
			SourceId:   sourceIdStr,
			Content:    doc.Content,
			Owner:      source.OwnerId,
			Embedding:  slices.CastFloat[float64, float32](embeddings[pos]),
			ChunkPos:   int32(pos),
		}
		fallbackVsDocs = append(fallbackVsDocs, vdoc)

		leafNodes = append(leafNodes, indices.NewDocTreeNode(vdoc, 0, pos, nil, []string{doc.ID}))
		idPosMapping[doc.ID] = pos
	}

	// 构建索引树
	docTree, err := b.docTreeBuilder.MergeBuild(ctx, leafNodes)
	if err != nil {
		// log only
		slog.ErrorContext(ctx, "build doc tree failed",
			slog.Any("err", err),
			slog.String("source_id", source.Id.String()),
		)

		return texts, fallbackVsDocs, nil
	} else {
		nodes := docTree.Nodes()

		for _, node := range nodes {
			vDoc := node.Core()
			vsDocs = append(vsDocs, vDoc)

			if node.IsLeaf() {
				continue
			}

			// 派生节点需要额外处理
			if derivingIds := node.DerivedFrom(); len(derivingIds) > 0 {
				bm := bitmap.New(uint32(docsLen))
				for _, derivingId := range derivingIds {
					bitPos, ok := idPosMapping[derivingId]
					if ok {
						bm.Set(uint32(bitPos))
					}
				}
				childrenPos := make([]int, 0, len(node.Children()))
				for _, child := range node.Children() {
					// children_pos 必须和 RecoverDocTree 的查找键一致，使用 chunk_pos/pos 体系。
					childrenPos = append(childrenPos, child.Pos())
				}

				vDoc.PutMeta(sourceDocMetaDerivingPos, bm.String())
				vDoc.PutMeta(sourceDocMetaLevel, int64(node.Level()))
				vDoc.PutMeta(sourceDocMetaChildrenPos, childrenPos)
			}
		}

		slog.DebugContext(ctx, "build doc tree success",
			slog.Int("node_count", len(nodes)),
			slog.String("root_summary", docTree.Root().Core().GetContent()),
			slog.Int("tree_height", docTree.Height()),
			slog.String("source_id", source.Id.String()),
		)
	}

	return texts, vsDocs, nil
}

// 插入向量库
func (b *SourceIndexer) insertSourceDocs(
	ctx context.Context,
	vsDocs []*vecschema.SourceDoc,
) error {
	err := b.sourceDocStore.BatchInsert(ctx, vsDocs)
	if err != nil {
		return errors.WithMessagef(err, "insert source docs failed")
	}

	return nil
}

func RecoverDocTree(ctx context.Context, docs []*vecschema.SourceDoc) (*indices.DocTree, error) {
	_ = ctx
	if len(docs) == 0 {
		return nil, errors.ErrParams.Msg("no source docs found")
	}

	// root 是最小 chunk_pos，先排序保证重建入口稳定。
	sortedDocs := append(make([]*vecschema.SourceDoc, 0, len(docs)), docs...)
	stdslices.SortFunc(sortedDocs, func(a, b *vecschema.SourceDoc) int {
		return int(a.ChunkPos - b.ChunkPos)
	})
	rootDoc := sortedDocs[0]

	docPosMapping := make(map[int]*vecschema.SourceDoc, len(sortedDocs))
	for _, doc := range sortedDocs {
		docPosMapping[int(doc.ChunkPos)] = doc
	}

	builder := &docTreeRecoverBuilder{
		docPosMapping: docPosMapping,
		nodeByPos:     make(map[int]*indices.DocTreeNode, len(sortedDocs)),
		building:      make(map[int]bool, len(sortedDocs)),
	}
	rootNode, err := builder.buildNode(int(rootDoc.ChunkPos))
	if err != nil {
		return nil, errors.WithMessagef(err, "build root node failed, root_pos=%d", rootDoc.ChunkPos)
	}

	rebuiltNodes := make([]*indices.DocTreeNode, 0, len(sortedDocs))
	maxLevel := 0
	for _, doc := range sortedDocs {
		pos := int(doc.ChunkPos)
		node, ok := builder.nodeByPos[pos]
		if !ok {
			return nil, errors.ErrNoRecord.Msgf("orphan node found, node_pos=%d", pos)
		}
		rebuiltNodes = append(rebuiltNodes, node)
		if node.Level() > maxLevel {
			maxLevel = node.Level()
		}
	}

	docTree := &indices.DocTree{}
	docTree.SetRoot(rootNode)
	docTree.SetNodes(rebuiltNodes)
	docTree.SetHeight(maxLevel + 1)

	return docTree, nil
}

type docTreeRecoverBuilder struct {
	docPosMapping map[int]*vecschema.SourceDoc
	nodeByPos     map[int]*indices.DocTreeNode
	building      map[int]bool
}

func (b *docTreeRecoverBuilder) buildNode(pos int) (*indices.DocTreeNode, error) {
	if n, ok := b.nodeByPos[pos]; ok {
		return n, nil
	}
	if b.building[pos] {
		return nil, errors.ErrInner.Msgf("cycle detected while rebuilding doc tree, node_pos=%d", pos)
	}

	doc, ok := b.docPosMapping[pos]
	if !ok {
		return nil, errors.ErrNoRecord.Msgf("doc not found for node pos, node_pos=%d", pos)
	}

	isLeaf := pos >= 0
	level := 0
	if lv, ok := doc.GetMetaInt(sourceDocMetaLevel); ok {
		level = lv
	}

	b.building[pos] = true
	defer delete(b.building, pos)

	var (
		children    []*indices.DocTreeNode
		derivedFrom []string
	)
	if isLeaf {
		derivedFrom = []string{doc.Id}
	} else {
		childrenPos, err := readSourceDocMetaIntSlice(doc, sourceDocMetaChildrenPos)
		if err != nil {
			return nil, errors.WithMessagef(err, "read children pos failed, node_pos=%d", pos)
		}
		if len(childrenPos) == 0 {
			return nil, errors.ErrInner.Msgf("children pos is empty for non-leaf node, node_pos=%d", pos)
		}
		children = make([]*indices.DocTreeNode, 0, len(childrenPos))
		for _, childPos := range childrenPos {
			childNode, err := b.buildNode(childPos)
			if err != nil {
				return nil, err
			}
			children = append(children, childNode)
		}

		derivedFrom, err = readSourceDocDerivedFrom(doc, b.docPosMapping)
		if err != nil {
			return nil, errors.WithMessagef(err, "read deriving docs failed, node_pos=%d", pos)
		}
	}

	node := indices.NewDocTreeNode(doc, level, pos, children, derivedFrom)
	b.nodeByPos[pos] = node
	return node, nil
}

func readSourceDocMetaIntSlice(doc *vecschema.SourceDoc, key string) ([]int, error) {
	if doc == nil {
		return nil, errors.ErrParams.Msg("source doc is nil")
	}
	out, ok, err := doc.GetMetaIntSlice(key)
	if !ok {
		return nil, errors.ErrInner.Msgf("meta key not found, key=%s", key)
	}
	if err != nil {
		return nil, errors.WithMessagef(err, "cast children pos failed, key=%s", key)
	}
	return out, nil
}

func readSourceDocDerivedFrom(
	doc *vecschema.SourceDoc,
	docPosMapping map[int]*vecschema.SourceDoc,
) ([]string, error) {
	if doc == nil {
		return nil, errors.ErrParams.Msg("source doc is nil")
	}
	encoded, ok := doc.GetStringMeta(sourceDocMetaDerivingPos)
	if !ok || encoded == "" {
		return nil, errors.ErrInner.Msgf("meta deriving pos not found, node_id=%s", doc.Id)
	}
	bm, err := bitmap.NewFrom(encoded)
	if err != nil {
		return nil, errors.WithMessagef(err, "decode deriving bitmap failed, node_id=%s", doc.Id)
	}

	setPos := bm.GetAllSet()
	if len(setPos) == 0 {
		return nil, errors.ErrInner.Msgf("deriving bitmap has no set bit, node_id=%s", doc.Id)
	}
	derivedFrom := make([]string, 0, len(setPos))
	for _, leafPos := range setPos {
		leafDoc, ok := docPosMapping[int(leafPos)]
		if !ok {
			return nil, errors.ErrNoRecord.Msgf("leaf doc not found by deriving bitmap, leaf_pos=%d", leafPos)
		}
		derivedFrom = append(derivedFrom, leafDoc.Id)
	}

	return slices.Unique(derivedFrom), nil
}
