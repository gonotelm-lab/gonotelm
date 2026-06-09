package indices

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"
	"unsafe"

	sourceutil "github.com/gonotelm-lab/gonotelm/internal/app/biz/source/util"
	vschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/chunker/recursive"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/token"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	goldtext "github.com/yuin/goldmark/text"

	einoschema "github.com/cloudwego/eino/schema"
)

type parseMetadata struct {
	startByte int
	endByte   int
	startRune int
	endRune   int
}

func (m *parseMetadata) StartByte() int {
	if m == nil {
		return 0
	}
	return m.startByte
}

func (m *parseMetadata) EndByte() int {
	if m == nil {
		return 0
	}
	return m.endByte
}

func (m *parseMetadata) StartRune() int {
	if m == nil {
		return 0
	}
	return m.startRune
}

func (m *parseMetadata) EndRune() int {
	if m == nil {
		return 0
	}
	return m.endRune
}

func (m *parseMetadata) Valid() bool {
	if m == nil {
		return false
	}
	return m.endByte > m.startByte && m.endRune >= m.startRune
}

type markdownByteRange struct {
	start int
	end   int
}

type markdownDocTreeNode struct {
	id           string
	title        string
	contents     []string
	children     []*markdownDocTreeNode
	headingLevel int // 此处根节点是level=0
	pos          int

	derivation []string

	derived bool

	parseMetadata *parseMetadata
	contentRanges []markdownByteRange
}

type ParseBuildChunk struct {
	Content string
	// StartByte/EndByte 是相对于本次 split 输入 content 的 byte 偏移（左闭右开）。
	// 当 splitter 无法提供 span 时可返回 <0，后续会跳过位置信息注入。
	StartByte int
	EndByte   int
}

type ParseBuildChunkSplitFunc func(ctx context.Context, content string) ([]ParseBuildChunk, error)

type ParseBuildOption func(*parseBuildOptions)

type parseBuildOptions struct {
	maxNodeToken int
	overlapToken int
	separators   []string
	tokenLenFn   func(string) int
	chunkSplitFn ParseBuildChunkSplitFunc

	embedBatchSize      int
	embedMaxConcurrency int
}

const (
	defaultParseBuildMaxNodeToken = 800
)

var defaultParseBuildSeparators = []string{
	"\n\n", "\n", ".", "?", "!", "。", "？", "！", ";", "；", " ",
}

func defaultParseBuildOptions() *parseBuildOptions {
	return &parseBuildOptions{
		maxNodeToken: defaultParseBuildMaxNodeToken,
		overlapToken: defaultParseBuildMaxNodeToken / 6,
		separators:   append([]string(nil), defaultParseBuildSeparators...),
		tokenLenFn:   token.Estimate,
	}
}

func WithParseBuildMaxNodeToken(maxToken int) ParseBuildOption {
	return func(opt *parseBuildOptions) {
		if maxToken > 0 {
			opt.maxNodeToken = maxToken
		}
	}
}

func WithParseBuildOverlapToken(overlapToken int) ParseBuildOption {
	return func(opt *parseBuildOptions) {
		if overlapToken >= 0 {
			opt.overlapToken = overlapToken
		}
	}
}

func WithParseBuildSplitSeparators(separators []string) ParseBuildOption {
	return func(opt *parseBuildOptions) {
		if len(separators) > 0 {
			opt.separators = append([]string(nil), separators...)
		}
	}
}

func WithParseBuildTokenLenFn(tokenLenFn func(string) int) ParseBuildOption {
	return func(opt *parseBuildOptions) {
		if tokenLenFn != nil {
			opt.tokenLenFn = tokenLenFn
		}
	}
}

func WithParseBuildChunkSplitFunc(chunkSplitFn ParseBuildChunkSplitFunc) ParseBuildOption {
	return func(opt *parseBuildOptions) {
		if chunkSplitFn != nil {
			opt.chunkSplitFn = chunkSplitFn
		}
	}
}

func WithParseBuildEmbedBatch(batchSize int, maxConcurrency int) ParseBuildOption {
	return func(opt *parseBuildOptions) {
		if batchSize > 0 {
			opt.embedBatchSize = batchSize
		}
		if maxConcurrency > 0 {
			opt.embedMaxConcurrency = maxConcurrency
		}
	}
}

func buildParseBuildOptions(opts ...ParseBuildOption) (*parseBuildOptions, error) {
	cfg := defaultParseBuildOptions()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(cfg)
	}
	if cfg.maxNodeToken <= 0 {
		cfg.maxNodeToken = defaultParseBuildMaxNodeToken
	}
	if cfg.overlapToken < 0 {
		cfg.overlapToken = 0
	}
	if cfg.overlapToken >= cfg.maxNodeToken {
		cfg.overlapToken = cfg.maxNodeToken / 6
	}
	if cfg.tokenLenFn == nil {
		cfg.tokenLenFn = token.Estimate
	}
	if len(cfg.separators) == 0 {
		cfg.separators = append([]string(nil), defaultParseBuildSeparators...)
	}
	if cfg.chunkSplitFn == nil {
		// 默认 splitter：返回 chunk 文本 + 相对 byte span，供 ParseBuild 直接映射到原文 offset，
		// 避免在 parsebuild 内做二次字符串回扫定位。
		splitter, err := recursive.NewSplitter(context.Background(),
			&recursive.Config{
				ChunkSize:   cfg.maxNodeToken,
				OverlapSize: cfg.overlapToken,
				LenFunc:     cfg.tokenLenFn,
				KeepType:    recursive.KeepTypeEnd,
				Separators:  cfg.separators,
			})
		if err != nil {
			return nil, errors.Wrapf(errors.ErrInner, "create recursive splitter failed, err=%v", err)
		}
		cfg.chunkSplitFn = func(ctx context.Context, content string) ([]ParseBuildChunk, error) {
			docs, err := splitter.Transform(ctx, []*einoschema.Document{
				{ID: "parsebuild", Content: content},
			})
			if err != nil {
				return nil, errors.Wrapf(errors.ErrInner,
					"split content by recursive splitter failed, err=%v", err)
			}

			chunks := make([]string, 0, len(docs))
			for _, doc := range docs {
				chunks = append(chunks, doc.Content)
			}
			chunkSpans := sourceutil.BuildChunkByteSpans(content, chunks)
			splitChunks := make([]ParseBuildChunk, 0, len(chunks))
			for idx, chunk := range chunks {
				span := chunkSpans[idx]
				splitChunks = append(splitChunks, ParseBuildChunk{
					Content:   chunk,
					StartByte: span.StartByte,
					EndByte:   span.EndByte,
				})
			}

			return splitChunks, nil
		}
	}

	return cfg, nil
}

// ParseBuild 基于 markdown AST 构建可检索的文档树（DocTree）。
//
// 执行步骤：
// 1) 解析 markdown 为 AST，并按 Heading 层级构建临时树。
// 2) 同步采集每个节点在原文中的 byte span（标题 + 正文块）。
// 3) 对超 token 节点做分裂（叶子横向分裂、非叶正文下放），并保留 span 映射关系。
// 4) 统一将 byte offset 转成 rune offset，产出 parseMetadata。
// 5) 组装 DocTreeNode 并执行 embedding。
//
// 关键原理：
// - 位置信息遵循“原生节点优先”：派生节点（如 root 摘要）不注入 parseMetadata。
// - split 接口返回 chunk+span（相对 split 输入内容），避免二次字符串回扫。
// - 当 span 无法安全映射时，跳过该节点的位置信息而不阻塞主流程。
//
// 注意：
// - content 为空直接返回参数错误。
// - 若 LLM root 摘要、分裂或 embedding 失败，按阶段返回对应错误。
func (b *DocTreeBuilder) ParseBuild(
	ctx context.Context,
	content []byte,
	opts ...ParseBuildOption,
) (*DocTree, error) {
	if len(content) == 0 {
		return nil, errors.ErrParams.Msg("parse build content is empty")
	}
	buildOptions, err := buildParseBuildOptions(opts...)
	if err != nil {
		return nil, err
	}

	// 1) 先把 markdown 解析为 AST。
	parser := goldmark.DefaultParser()
	reader := goldtext.NewReader(content)
	markdoc := parser.Parse(reader)

	// 2) 遍历顶层节点，构建按标题组织的文档树，同时收集原文 byte span。
	vroot := &markdownDocTreeNode{headingLevel: 0, title: "vroot"}
	stack := []*markdownDocTreeNode{vroot}

	// 栈顶放的节点为可能的父节点
	pushStack := func(node *markdownDocTreeNode) {
		stack = append(stack, node)
	}
	peekStack := func() *markdownDocTreeNode {
		return stack[len(stack)-1]
	}
	popStack := func() *markdownDocTreeNode {
		last := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return last
	}

	curParent := vroot

	for n := markdoc.FirstChild(); n != nil; n = n.NextSibling() {
		switch node := n.(type) {
		case *ast.Heading: // 只识别heading 其它一律归结为heading下的内容
			// H1~H6
			// 找到比当前节点Level更小的节点 也就是找到其父节点
			for peekStack().headingLevel >= node.Level {
				popStack()
			}

			parent := peekStack()
			newNode := &markdownDocTreeNode{
				headingLevel: node.Level,
				title:        extractMarkdownInlineText(node, content),
			}
			if br, ok := extractNodeByteRange(node, content); ok {
				mergeNodeParseMeta(newNode, br)
			}

			parent.children = append(parent.children, newNode)
			pushStack(newNode) // 新的节点可能作为其他节点的父节点
			curParent = newNode
		default:
			// 非标题块全部挂到当前标题节点下，内容与 span 同步累计。
			text := extractMarkdownBlockText(node, content)
			if text != "" {
				curParent.contents = append(curParent.contents, text)
				if br, ok := extractNodeByteRange(node, content); ok {
					appendNodeContentRange(curParent, text, br, content)
				}
			}
		}
	}

	err = b.generateRootTitle(ctx, vroot)
	if err != nil {
		return nil, errors.Wrap(err, "generate vroot title failed")
	}

	// 3) 对超限节点做递归分裂（会保留/下放结构并同步 span）。
	rootList, err := b.splitOversizedNode(ctx, vroot, buildOptions, true, content)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "split oversized markdown tree node failed, err=%v", err)
	}

	// 4) 稀疏收集需要转换的 byte offset，一次扫描计算 rune offset，避免全量 []int 索引。
	var byteOffsets []int
	for _, rootNode := range rootList {
		if rootNode == nil {
			continue
		}
		byteOffsets = collectNodeByteOffsets(rootNode, byteOffsets)
	}
	runeOffsetMap := computeRuneOffsetsFromBytes(content, byteOffsets)
	for _, rootNode := range rootList {
		if rootNode == nil {
			continue
		}
		applyNodeRuneOffsetFromMap(rootNode, runeOffsetMap)
	}

	root, nodes := buildDocTreeFromNode(rootList[0])

	err = b.embedDocTreeNodes(ctx, nodes, buildOptions)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "embed parse-build nodes failed, err=%v", err)
	}

	return &DocTree{
		root:   root,
		nodes:  nodes,
		height: root.level + 1,
	}, nil
}

func (b *DocTreeBuilder) generateRootTitle(ctx context.Context, vroot *markdownDocTreeNode) error {
	titles := collectNodeTitles(vroot.children)
	if len(titles) == 0 {
		// 无标题正文：root 仍是原文节点，不属于派生摘要节点。
		vroot.title = ""
		vroot.derived = false
		return nil
	}
	titleContent := strings.Join(slices.Unique(titles), "\n")

	summary, err := b.summarizer.Summarize(ctx, titleContent)
	if err != nil {
		return errors.WithMessagef(err, "generate vroot summary failed")
	}
	if summary == "" {
		summary = "vroot"
	}
	vroot.title = summary
	// 有标题场景下，root title 由 LLM 摘要生成，属于派生信息。
	vroot.derived = true

	return nil
}

func collectNodeTitles(nodes []*markdownDocTreeNode) []string {
	return collectNodeTitlesInto(nodes, nil)
}

func collectNodeTitlesInto(nodes []*markdownDocTreeNode, acc []string) []string {
	for _, node := range nodes {
		if title := strings.TrimSpace(node.title); title != "" {
			acc = append(acc, title)
		}
		acc = collectNodeTitlesInto(node.children, acc)
	}
	return acc
}

func extractNodeByteRange(node ast.Node, source []byte) (markdownByteRange, bool) {
	if node == nil || len(source) == 0 {
		return markdownByteRange{}, false
	}

	lines := node.Lines()
	if lines == nil || lines.Len() == 0 {
		return markdownByteRange{}, false
	}

	start := lines.At(0).Start
	end := lines.At(lines.Len() - 1).Stop
	return normalizeByteRange(markdownByteRange{start: start, end: end}, len(source))
}

func normalizeByteRange(br markdownByteRange, contentLen int) (markdownByteRange, bool) {
	if contentLen <= 0 {
		return markdownByteRange{}, false
	}
	if br.start < 0 {
		br.start = 0
	}
	if br.end > contentLen {
		br.end = contentLen
	}
	if br.start >= br.end {
		return markdownByteRange{}, false
	}
	return br, true
}

func mergeNodeParseMeta(node *markdownDocTreeNode, br markdownByteRange) {
	if node == nil {
		return
	}
	node.parseMetadata = mergeMetaByteRange(node.parseMetadata, br)
}

func appendNodeContentRange(
	node *markdownDocTreeNode,
	content string,
	br markdownByteRange,
	source []byte,
) {
	if node == nil {
		return
	}
	// 优先将 AST block 粗粒度 span 对齐到 extract 后文本的精确 span，
	// 降低后续 split span 映射误差。
	if aligned, ok := alignByteRangeByContent(content, br, source); ok {
		br = aligned
	}
	node.contentRanges = append(node.contentRanges, br)
	mergeNodeParseMeta(node, br)
}

func alignByteRangeByContent(
	content string,
	br markdownByteRange,
	source []byte,
) (markdownByteRange, bool) {
	if content == "" || len(source) == 0 {
		return markdownByteRange{}, false
	}
	scoped, ok := normalizeByteRange(br, len(source))
	if !ok {
		return markdownByteRange{}, false
	}

	scope := source[scoped.start:scoped.end]
	scopeText := unsafe.String(unsafe.SliceData(scope), len(scope))
	idx := strings.Index(scopeText, content)
	if idx < 0 {
		return markdownByteRange{}, false
	}
	alignedStart := scoped.start + idx
	alignedEnd := alignedStart + len(content)
	aligned, ok := normalizeByteRange(
		markdownByteRange{
			start: alignedStart,
			end:   alignedEnd,
		},
		len(source),
	)
	if !ok {
		return markdownByteRange{}, false
	}
	return aligned, true
}

func mergeMetaByteRange(meta *parseMetadata, br markdownByteRange) *parseMetadata {
	if meta == nil {
		return &parseMetadata{
			startByte: br.start,
			endByte:   br.end,
		}
	}
	if br.start < meta.startByte {
		meta.startByte = br.start
	}
	if br.end > meta.endByte {
		meta.endByte = br.end
	}
	return meta
}

func cloneParseMetadata(meta *parseMetadata) *parseMetadata {
	if meta == nil {
		return nil
	}
	cloned := *meta
	return &cloned
}

func normalizeChunks(chunks []ParseBuildChunk) []ParseBuildChunk {
	result := make([]ParseBuildChunk, 0, len(chunks))
	for _, chunk := range chunks {
		// splitter 可能保留首尾换行；此处做同一化，避免内容和 span 不一致。
		leftTrim := len(chunk.Content) - len(strings.TrimLeft(chunk.Content, "\n\r"))
		rightTrim := len(chunk.Content) - len(strings.TrimRight(chunk.Content, "\n\r"))
		trimmed := strings.Trim(chunk.Content, "\n\r")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}

		normalized := ParseBuildChunk{
			Content:   trimmed,
			StartByte: -1,
			EndByte:   -1,
		}
		if chunk.StartByte >= 0 && chunk.EndByte >= chunk.StartByte {
			normalized.StartByte = chunk.StartByte + leftTrim
			normalized.EndByte = chunk.EndByte - rightTrim
			if normalized.EndByte <= normalized.StartByte {
				// span 非法时标记为 unknown，后续直接跳过位置注入而不影响主流程。
				normalized.StartByte = -1
				normalized.EndByte = -1
			}
		}
		result = append(result, normalized)
	}

	return result
}

func chunkToSourceRange(
	node *markdownDocTreeNode,
	source []byte,
	chunk ParseBuildChunk,
) (markdownByteRange, bool) {
	// 目标：把 chunk（相对 join 后内容的 span）映射回原文 source 的 byte span。
	if node == nil || len(source) == 0 {
		return markdownByteRange{}, false
	}
	if chunk.StartByte < 0 || chunk.EndByte <= chunk.StartByte {
		return markdownByteRange{}, false
	}
	// 仅当 contents 与 contentRanges 一一对应时才能做可逆映射。
	if len(node.contents) == 0 || len(node.contentRanges) == 0 || len(node.contents) != len(node.contentRanges) {
		return markdownByteRange{}, false
	}

	// chunk span 是相对于 joinNodeContent(node.contents) 的；
	// 先校验该 span 是否落在 join 后内容范围内。
	joinedLen := 0
	for idx, contentPart := range node.contents {
		rangeLen := node.contentRanges[idx].end - node.contentRanges[idx].start
		if rangeLen < len(contentPart) {
			return markdownByteRange{}, false
		}
		// joinNodeContent 在块间额外插入 '\n'，因此非最后一块额外 +1。
		joinedLen += len(contentPart)
		if idx < len(node.contents)-1 {
			joinedLen++
		}
	}
	if chunk.EndByte > joinedLen {
		return markdownByteRange{}, false
	}

	startByte, ok := joinedOffsetToSourceByte(node.contents, node.contentRanges, chunk.StartByte, false)
	if !ok {
		return markdownByteRange{}, false
	}
	// endByte 映射使用 isEndBoundary=true，确保落在拼接 '\n' 上时归属到前一块结尾。
	endByte, ok := joinedOffsetToSourceByte(node.contents, node.contentRanges, chunk.EndByte, true)
	if !ok {
		return markdownByteRange{}, false
	}

	// 最后统一归一化到合法 source 边界内。
	return normalizeByteRange(
		markdownByteRange{
			start: startByte,
			end:   endByte,
		},
		len(source),
	)
}

// joinedOffsetToSourceByte 把 “join 后字符串的偏移” 映射回 “原文 byte 偏移”。
//
// 参数语义：
// - contents/ranges：一一对应，表示每个块的抽取文本和其在原文中的 byte 区间。
// - offset：基于 joinNodeContent(contents) 的偏移。
// - isEndBoundary：当 offset 正好落在块间拼接的 '\n' 上时，用于区分是“起始边界”还是“结束边界”。
//
// 例子：contents=["abc","def"]，join 后为 "abc\ndef"。
// - offset=3（落在 '\n' 处）且 isEndBoundary=false -> 映射到第二块起点；
// - offset=3 且 isEndBoundary=true  -> 映射到第一块终点。
func joinedOffsetToSourceByte(
	contents []string,
	ranges []markdownByteRange,
	offset int,
	isEndBoundary bool,
) (int, bool) {
	if offset < 0 || len(contents) == 0 || len(contents) != len(ranges) {
		return 0, false
	}

	acc := 0
	for idx, content := range contents {
		rangeLen := ranges[idx].end - ranges[idx].start
		contentLen := len(content)
		if rangeLen < contentLen {
			// 防御式检查：若抽取文本比原始区间还长，说明上游 span 对齐异常。
			return 0, false
		}

		// 当前块在 join 后内容中的闭区间 [segmentStart, segmentEnd]。
		segmentStart := acc
		segmentEnd := acc + contentLen
		if offset >= segmentStart && offset <= segmentEnd {
			delta := offset - segmentStart
			return ranges[idx].start + delta, true
		}
		acc = segmentEnd

		if idx == len(contents)-1 {
			break
		}

		// joinNodeContent 会在块间插入 '\n'：
		// - 对起始边界，映射到下一块开头；
		// - 对结束边界，映射到前一块结尾。
		if offset == acc {
			if isEndBoundary {
				return ranges[idx].end, true
			}
			return ranges[idx+1].start, true
		}
		// 跳过 join 时人为插入的 '\n'。
		acc++
	}

	if offset == acc {
		last := ranges[len(ranges)-1]
		return last.end, true
	}

	return 0, false
}

func buildSplitChunkNode(
	base *markdownDocTreeNode,
	source []byte,
	chunk ParseBuildChunk,
	title string,
	headingLevel int,
) *markdownDocTreeNode {
	var chunkParseMeta *parseMetadata
	if br, ok := chunkToSourceRange(base, source, chunk); ok {
		chunkParseMeta = &parseMetadata{
			startByte: br.start,
			endByte:   br.end,
		}
	}

	return &markdownDocTreeNode{
		title:         title,
		contents:      []string{chunk.Content},
		headingLevel:  headingLevel,
		derived:       false,
		parseMetadata: chunkParseMeta,
	}
}

func collectNodeByteOffsets(node *markdownDocTreeNode, offsets []int) []int {
	if node == nil {
		return offsets
	}
	if m := node.parseMetadata; m != nil && m.endByte > m.startByte {
		offsets = append(offsets, m.startByte, m.endByte)
	}
	for _, child := range node.children {
		offsets = collectNodeByteOffsets(child, offsets)
	}
	return offsets
}

func computeRuneOffsetsFromBytes(content []byte, byteOffsets []int) map[int]int {
	if len(byteOffsets) == 0 {
		return nil
	}

	sort.Ints(byteOffsets)
	// deduplicate in-place
	j := 0
	for i, v := range byteOffsets {
		if i == 0 || v != byteOffsets[i-1] {
			byteOffsets[j] = v
			j++
		}
	}
	byteOffsets = byteOffsets[:j]

	result := make(map[int]int, len(byteOffsets))
	runeIdx := 0
	nextTarget := 0

	for bytePos := 0; bytePos < len(content) && nextTarget < len(byteOffsets); {
		for nextTarget < len(byteOffsets) && byteOffsets[nextTarget] <= bytePos {
			result[byteOffsets[nextTarget]] = runeIdx
			nextTarget++
		}
		_, size := utf8.DecodeRune(content[bytePos:])
		bytePos += size
		runeIdx++
	}
	for nextTarget < len(byteOffsets) {
		result[byteOffsets[nextTarget]] = runeIdx
		nextTarget++
	}

	return result
}

func applyNodeRuneOffsetFromMap(node *markdownDocTreeNode, runeOffsetMap map[int]int) {
	if node == nil {
		return
	}

	if node.parseMetadata != nil {
		if node.parseMetadata.endByte <= node.parseMetadata.startByte {
			node.parseMetadata = nil
		} else {
			node.parseMetadata.startRune = runeOffsetMap[node.parseMetadata.startByte]
			node.parseMetadata.endRune = runeOffsetMap[node.parseMetadata.endByte]
		}
	}

	for _, child := range node.children {
		applyNodeRuneOffsetFromMap(child, runeOffsetMap)
	}
}

func (b *DocTreeBuilder) splitOversizedNode(
	ctx context.Context,
	node *markdownDocTreeNode,
	opt *parseBuildOptions,
	isRoot bool,
	source []byte,
) ([]*markdownDocTreeNode, error) {
	// 步骤A：先分裂子节点再处理当前节点。
	// 原因：当前节点是否还能保留/是否需要上提，依赖“子节点最终形态”，
	// 不能基于分裂前的旧 children 做决策。
	children := make([]*markdownDocTreeNode, 0, len(node.children))
	for _, child := range node.children {
		newChildren, err := b.splitOversizedNode(ctx, child, opt, false, source)
		if err != nil {
			return nil, err
		}
		children = append(children, newChildren...)
	}
	node.children = children

	content := joinNodeContent(node.contents)
	// 步骤B：压缩空节点（仅非根）。
	// - 空叶子：直接删除（返回 nil）。
	// - 空非叶：上提其 children，避免无意义中间层。
	// - 根节点：即使为空也保留，保证树入口稳定。
	if !isRoot && !hasEmbeddingContent(node.title, content) {
		if len(node.children) == 0 {
			return nil, nil
		}
		return node.children, nil
	}
	// 步骤C：无需分裂直接返回（无内容或 token 未超限）。
	if content == "" || !needSplitNode(content, opt) {
		return []*markdownDocTreeNode{node}, nil
	}

	// 步骤D：执行 split，并做 chunk 规范化（trim 首尾换行/过滤空块）。
	chunks, err := opt.chunkSplitFn(ctx, content)
	if err != nil {
		return nil, err
	}
	chunks = normalizeChunks(chunks)
	if len(chunks) <= 1 {
		return []*markdownDocTreeNode{node}, nil
	}

	// 步骤E1：叶子节点 -> 横向分裂。
	// 行为：每个 chunk 变成“同层兄弟节点”（保持原 title 和 headingLevel）。
	// 目的：避免新增中间层，保持检索结构扁平。
	if len(node.children) == 0 && !isRoot {
		siblings := make([]*markdownDocTreeNode, 0, len(chunks))
		for _, chunk := range chunks {
			siblings = append(siblings, buildSplitChunkNode(
				node,
				source,
				chunk,
				node.title,
				node.headingLevel,
			))
		}
		return siblings, nil
	}

	// 步骤E2：非叶子节点 -> 内容下放。
	// 行为：当前节点保留 title/children，原 contents 被切块后下放为“前置子节点”。
	// 目的：保留目录语义（标题层级）同时缩短每个可嵌入单元。
	chunkNodes := make([]*markdownDocTreeNode, 0, len(chunks))
	for _, chunk := range chunks {
		chunkNodes = append(chunkNodes, buildSplitChunkNode(
			node,
			source,
			chunk,
			"",
			node.headingLevel+1,
		))
	}
	node.contents = nil
	node.contentRanges = nil
	// 新切出的内容块排在已有 children 前，保证“父节点正文优先于子标题内容”。
	node.children = append(chunkNodes, node.children...)
	return []*markdownDocTreeNode{node}, nil
}

func needSplitNode(content string, opt *parseBuildOptions) bool {
	return opt.tokenLenFn(content) > opt.maxNodeToken
}

func joinNodeContent(contents []string) string {
	if len(contents) == 0 {
		return ""
	}
	joined := strings.Join(contents, "\n")
	if strings.TrimSpace(joined) == "" {
		return ""
	}
	return joined
}

func buildDocTreeFromNode(
	root *markdownDocTreeNode,
) (*DocTreeNode, []*DocTreeNode) {
	type buildState struct {
		nonDerivedPos int
		derivedPos    int
		nodes         []*DocTreeNode
	}

	state := &buildState{
		nonDerivedPos: 0,
		derivedPos:    -1,
		nodes:         make([]*DocTreeNode, 0),
	}

	var build func(node *markdownDocTreeNode) *DocTreeNode
	build = func(node *markdownDocTreeNode) *DocTreeNode {
		children := make([]*DocTreeNode, 0, len(node.children))
		childDerivation := make([]string, 0)
		maxChildLevel := 0
		for _, child := range node.children {
			childNode := build(child)
			children = append(children, childNode)
			childDerivation = append(childDerivation, childNode.derivation...)
			if childNode.level > maxChildLevel {
				maxChildLevel = childNode.level
			}
		}

		id := uuid.NewV4().String()
		node.id = id
		content := nodeEmbeddingContent(node.title, joinNodeContent(node.contents))
		core := &vschema.SourceDoc{
			Id:       id,
			Content:  content,
			ChunkPos: -1,
		}

		derivations := []string{id}
		if node.derived {
			derivations = slices.Unique(childDerivation)
		}
		node.derivation = derivations

		level := 0
		if len(children) > 0 {
			level = maxChildLevel + 1
		}
		pos := state.derivedPos
		if node.derived {
			state.derivedPos--
		} else {
			pos = state.nonDerivedPos
			state.nonDerivedPos++
		}

		docNode := NewDocTreeNode(core, level, pos, children, derivations)
		if !node.derived {
			docNode.parseMetadata = cloneParseMetadata(node.parseMetadata)
			if docNode.parseMetadata != nil && !docNode.parseMetadata.Valid() {
				docNode.parseMetadata = nil
			}
		}
		core.ChunkPos = int32(pos)
		state.nodes = append(state.nodes, docNode)
		return docNode
	}

	return build(root), state.nodes
}

func nodeEmbeddingContent(title string, content string) string {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	switch {
	case title == "":
		return content
	case content == "":
		return title
	default:
		var sb strings.Builder
		sb.Grow(len(title) + 2 + len(content))
		sb.WriteString(title)
		sb.WriteString("\n\n")
		sb.WriteString(content)
		return sb.String()
	}
}

func hasEmbeddingContent(title string, content string) bool {
	return strings.TrimSpace(title) != "" || strings.TrimSpace(content) != ""
}

func (b *DocTreeBuilder) embedDocTreeNodes(
	ctx context.Context,
	nodes []*DocTreeNode,
	opt *parseBuildOptions,
) error {
	texts := make([]string, len(nodes))
	for idx, node := range nodes {
		texts[idx] = node.core.Content
	}
	batchSize, maxConcurrency := resolveEmbedBatchSettings(len(texts), opt)
	embedResp, err := batch.ParallelMap(
		ctx,
		texts,
		batchSize,
		maxConcurrency,
		func(ctx context.Context, batchTexts []string) ([][]float64, error) {
			return b.embedder.EmbedStrings(ctx, batchTexts)
		},
	)
	if err != nil {
		return errors.Wrapf(errors.ErrInner, "embed parse-build node content failed, err=%v", err)
	}
	if len(embedResp) != len(texts) {
		return errors.Wrapf(
			errors.ErrSerde,
			"embed result count mismatch, expected=%d, actual=%d",
			len(texts),
			len(embedResp),
		)
	}

	for idx, node := range nodes {
		node.core.Embedding = slices.CastFloat[float64, float32](embedResp[idx])
	}

	return nil
}

func resolveEmbedBatchSettings(
	total int,
	opt *parseBuildOptions,
) (batchSize int, maxConcurrency int) {
	batchSize = opt.embedBatchSize
	if batchSize <= 0 || batchSize > total {
		batchSize = total
	}
	if batchSize <= 0 {
		batchSize = 1
	}

	maxConcurrency = opt.embedMaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}

	return batchSize, maxConcurrency
}
