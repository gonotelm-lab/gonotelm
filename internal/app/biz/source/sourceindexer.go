package source

import (
	"context"
	"log/slog"
	stdslices "slices"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc"
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/indices"
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/util"
	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/textgen/summarizer"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	vecschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	"github.com/gonotelm-lab/gonotelm/pkg/bitmap"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/token"

	einoembed "github.com/cloudwego/eino/components/embedding"
	einoschema "github.com/cloudwego/eino/schema"
)

// 构建来源的索引
//
// Parse -> Chunk Transform
type SourceIndexer struct {
	embedder            einoembed.Embedder
	embedBatchSize      int
	embedMaxConcurrency int
	sourceDocStore      vectordb.SourceDocStore

	sourceConverters map[model.SourceKind]convertdoc.Handler
	docTreeBuilder   *indices.DocTreeBuilder
}

func NewSourceIndexer(
	embedder einoembed.Embedder,
	sourceDocStore vectordb.SourceDocStore,
	objectStorage storage.Storage,
	llmGateway *gateway.Gateway,
	prompt *bizprompt.Prompt,
) *SourceIndexer {
	hc := convertdoc.HandlerConfig{
		ChunkSize:   conf.Global().Chunking.Size,
		OverlapSize: conf.Global().Chunking.OverlapSize,
	}
	if hc.OverlapSize == 0 || hc.OverlapSize > hc.ChunkSize {
		hc.OverlapSize = int(float64(hc.ChunkSize) * 0.15)
	}

	summarizer := summarizer.NewWithOption(
		llmGateway,
		summarizer.SummarizeOption{
			Provider: conf.Global().Logic.Source.ModelProvider,
			Model:    conf.Global().Logic.Source.Model,
		},
		prompt,
	)

	return &SourceIndexer{
		embedder:            embedder,
		embedBatchSize:      conf.Global().Embedding.BatchSize,
		embedMaxConcurrency: conf.Global().Embedding.MaxConcurrency,
		sourceDocStore:      sourceDocStore,
		sourceConverters: map[model.SourceKind]convertdoc.Handler{
			model.SourceKindText: convertdoc.NewTextHandler(hc),
			model.SourceKindUrl:  convertdoc.NewUrlHandler(hc),
			model.SourceKindFile: convertdoc.NewFileObjectHandler(hc, objectStorage),
		},
		docTreeBuilder: indices.NewDocTreeBuilder(embedder, summarizer),
	}
}

func (b *SourceIndexer) Prepare(
	ctx context.Context,
	source *model.Source,
) (*PrepareSourceIndicesResult, error) {
	slog.DebugContext(ctx, "prepare source indices, converting...", slog.String("source_id", source.Id.String()))
	result, skippedTransform, err := b.handleConvertSource(ctx, source)
	if err != nil {
		return nil, err
	}

	// 超过token限制的直接报错不处理
	estimatedToken := token.Estimate(pkgstring.FromBytes(result.ParsedContent))
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

	var (
		textChunks []string
		vsDocs     []*vecschema.SourceDoc
		log        string
	)
	// 对于有明显层级结构的markdown文档尝试使用语法树解析
	shouldParseEmbed := skippedTransform ||
		(result.ParsedContentType == model.MimeTypeMarkdown &&
			util.MaybeHasMarkdownHeadingBytes(result.ParsedContent))
	if shouldParseEmbed {
		textChunks, vsDocs, err = b.parseEmbedChunks(ctx, source, result)
		log = "parse embed"
	} else {
		textChunks, vsDocs, err = b.mergeEmbedChunks(ctx, source, result)
		log = "merge embed"
	}
	if err != nil {
		return nil, errors.WithMessagef(err, "%s chunks failed", log)
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

func (b *SourceIndexer) handleConvertSource(
	ctx context.Context,
	source *model.Source,
) (*convertdoc.HandleResult, bool, error) {
	converter, ok := b.sourceConverters[source.Kind]
	if !ok {
		return nil, false, errors.ErrParams.Msgf("can not convert source for kind %s", source.Kind)
	}

	skippedTransform := false
	result, err := converter.Handle(
		ctx,
		source,
		convertdoc.WithHandleSkipTransformIf(func(
			source *model.Source,
			parsedDocs []*einoschema.Document,
			parsedContent []byte,
		) bool {
			// 如果解析出来的是markdown就跳过transform分块 交由indexer后续步骤进行分块处理
			ok := util.MaybeHasMarkdownHeadingBytes(parsedContent)
			skippedTransform = ok
			return ok
		}),
	)
	if err != nil {
		return nil, false, errors.WithMessagef(err, "embed source failed")
	}

	return result, skippedTransform, nil
}

func (b *SourceIndexer) parseEmbedChunks(
	ctx context.Context,
	source *model.Source,
	result *convertdoc.HandleResult,
) ([]string, []*vecschema.SourceDoc, error) {
	tree, err := b.docTreeBuilder.ParseBuild(
		ctx,
		result.ParsedContent,
		indices.WithParseBuildEmbedBatch(b.embedBatchSize, b.embedMaxConcurrency),
	)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "parse build failed")
	}

	nodes := tree.Nodes()
	chunkTexts := make([]string, 0, len(tree.Nodes()))
	sourceDocs := make([]*vecschema.SourceDoc, 0, len(tree.Nodes()))
	for _, node := range nodes {
		chunkTexts = append(chunkTexts, node.Core().Content)
		sourceDocs = append(sourceDocs, node.Core())
		// 补上每个doc的元数据
		node.Core().NotebookId = source.NotebookId.String()
		node.Core().SourceId = source.Id.String()
		node.Core().Owner = source.OwnerId
		writeNodeLevelAndTreeMeta(node)
		if parseMeta := node.ParseMetadata(); parseMeta != nil && parseMeta.Valid() {
			node.Core().PutMeta(model.ChunkMetaPosStartKey, parseMeta.StartRune())
			node.Core().PutMeta(model.ChunkMetaPosEndKey, parseMeta.EndRune())
			node.Core().PutMeta(model.ChunkMetaPosByteStartKey, parseMeta.StartByte())
			node.Core().PutMeta(model.ChunkMetaPosByteEndKey, parseMeta.EndByte())
		}
	}
	// deriving bitmap 按“非派生节点 pos 体系”编码。
	nonDerivedIDPosMapping, bitmapSize := buildNonDerivedIDPosMapping(nodes)

	// 设置metadata
	for _, node := range nodes {
		if node.IsLeaf() {
			continue
		}
		if derivingIds := node.Derivation(); len(derivingIds) > 0 {
			if bitmapSize == 0 {
				continue
			}
			derivingIDs := slices.Unique(derivingIds)
			bm := bitmap.New(uint32(bitmapSize))
			hasSetBit := false
			for _, derivingID := range derivingIDs {
				bitPos, ok := nonDerivedIDPosMapping[derivingID]
				if ok {
					bm.Set(uint32(bitPos))
					hasSetBit = true
				}
			}
			if !hasSetBit {
				continue
			}
			node.Core().PutMeta(model.SourceDocMetaDerivingPos, bm.String())
			node.Core().PutMeta(model.SourceDocMetaChildrenPos, collectChildPoses(node))
		}
	}

	return chunkTexts, sourceDocs, nil
}

func (b *SourceIndexer) mergeEmbedChunks(
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
		// fallback 路径只有叶子节点，显式补 level=0，保证每个节点都有该元数据。
		vdoc.PutMeta(model.SourceDocMetaLevel, int64(0))
		if raw, ok := doc.MetaData[model.ChunkMetaPosStartKey]; ok {
			vdoc.PutMeta(model.ChunkMetaPosStartKey, raw)
		}
		if raw, ok := doc.MetaData[model.ChunkMetaPosEndKey]; ok {
			vdoc.PutMeta(model.ChunkMetaPosEndKey, raw)
		}
		if raw, ok := doc.MetaData[model.ChunkMetaPosByteStartKey]; ok {
			vdoc.PutMeta(model.ChunkMetaPosByteStartKey, raw)
		}
		if raw, ok := doc.MetaData[model.ChunkMetaPosByteEndKey]; ok {
			vdoc.PutMeta(model.ChunkMetaPosByteEndKey, raw)
		}
		fallbackVsDocs = append(fallbackVsDocs, vdoc)

		leafNodes = append(leafNodes, indices.NewDocTreeNode(vdoc, 0, pos, nil, []string{doc.ID}))
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
		nonDerivedIDPosMapping, bitmapSize := buildNonDerivedIDPosMapping(nodes)

		for _, node := range nodes {
			vDoc := node.Core()
			vsDocs = append(vsDocs, vDoc)
			writeNodeLevelAndTreeMeta(node)

			if node.IsLeaf() {
				continue
			}

			// 派生节点需要额外处理
			if derivingIds := node.Derivation(); len(derivingIds) > 0 {
				if bitmapSize == 0 {
					continue
				}
				derivingIDs := slices.Unique(derivingIds)
				bm := bitmap.New(uint32(bitmapSize))
				hasSetBit := false
				for _, derivingID := range derivingIDs {
					bitPos, ok := nonDerivedIDPosMapping[derivingID]
					if ok {
						bm.Set(uint32(bitPos))
						hasSetBit = true
					}
				}
				if !hasSetBit {
					continue
				}

				vDoc.PutMeta(model.SourceDocMetaDerivingPos, bm.String())
				vDoc.PutMeta(model.SourceDocMetaChildrenPos, collectChildPoses(node))
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

func buildNonDerivedIDPosMapping(nodes []*indices.DocTreeNode) (map[string]int, int) {
	out := make(map[string]int, len(nodes))
	maxPos := -1
	for _, node := range nodes {
		if !isNonDerivedNode(node) {
			continue
		}
		pos := node.Pos()
		if pos < 0 {
			continue
		}
		out[node.Core().Id] = pos
		if pos > maxPos {
			maxPos = pos
		}
	}
	return out, maxPos + 1
}

func writeNodeLevelAndTreeMeta(node *indices.DocTreeNode) {
	if node == nil || node.Core() == nil {
		return
	}

	core := node.Core()
	core.PutMeta(model.SourceDocMetaLevel, int64(node.Level()))
	// parent_pos 是 TreeMeta 关系链的关键入口：有父节点就写入，无父节点（root）保持缺失即可。
	if parent := node.Parent(); parent != nil {
		core.PutMeta(model.SourceDocMetaParentPos, parent.Pos())
	}
}

func collectChildPoses(node *indices.DocTreeNode) []int {
	if node == nil {
		return nil
	}

	childrenPos := make([]int, 0, len(node.Children()))
	for _, child := range node.Children() {
		// children_pos 必须和 RecoverDocTree 的查找键一致，使用 chunk_pos/pos 体系。
		childrenPos = append(childrenPos, child.Pos())
	}

	return childrenPos
}

func isNonDerivedNode(node *indices.DocTreeNode) bool {
	if node == nil || node.Core() == nil {
		return false
	}
	return node.Pos() >= 0
}

func isLeafNodeByChildrenMeta(doc *vecschema.SourceDoc) (bool, []int, error) {
	if doc == nil {
		return true, nil, nil
	}
	childrenPos, ok, err := doc.GetMetaIntSlice(model.SourceDocMetaChildrenPos)
	if err != nil {
		return false, nil, err
	}
	if !ok {
		return true, nil, nil
	}
	return false, childrenPos, nil
}

func recoverDocTree(ctx context.Context, docs []*vecschema.SourceDoc) (*indices.DocTree, error) {
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

	isLeaf, childrenPos, err := isLeafNodeByChildrenMeta(doc)
	if err != nil {
		return nil, errors.WithMessagef(err, "read children pos meta failed, node_pos=%d", pos)
	}
	level := 0
	if lv, ok := doc.GetMetaInt(model.SourceDocMetaLevel); ok {
		level = lv
	}

	b.building[pos] = true
	defer delete(b.building, pos)

	var (
		children   []*indices.DocTreeNode
		derivation []string
	)
	if isLeaf {
		derivation = []string{doc.Id}
	} else {
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

		derivation, err = readSourceDocDerivation(doc, b.docPosMapping)
		if err != nil {
			return nil, errors.WithMessagef(err, "read deriving docs failed, node_pos=%d", pos)
		}
	}

	node := indices.NewDocTreeNode(doc, level, pos, children, derivation)
	b.nodeByPos[pos] = node
	return node, nil
}

func readSourceDocDerivation(
	doc *vecschema.SourceDoc,
	docPosMapping map[int]*vecschema.SourceDoc,
) ([]string, error) {
	if doc == nil {
		return nil, errors.ErrParams.Msg("source doc is nil")
	}
	encoded, ok := doc.GetStringMeta(model.SourceDocMetaDerivingPos)
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
	derivation := make([]string, 0, len(setPos))
	for _, nonDerivedPos := range setPos {
		nonDerivedDoc, ok := docPosMapping[int(nonDerivedPos)]
		if !ok {
			return nil, errors.ErrNoRecord.Msgf(
				"non-derived doc not found by deriving bitmap, pos=%d",
				nonDerivedPos,
			)
		}
		derivation = append(derivation, nonDerivedDoc.Id)
	}

	return slices.Unique(derivation), nil
}
