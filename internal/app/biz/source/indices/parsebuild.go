package indices

import (
	"context"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
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

type markdownDocTreeNode struct {
	id           string
	title        string
	contents     []string
	children     []*markdownDocTreeNode
	headingLevel int // 此处根节点是level=0
	pos          int

	derivedFrom []string
}

type ParseBuildChunkSplitFunc func(ctx context.Context, content string) ([]string, error)

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
		tokenLenFn:   token.EstimateToken,
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
		cfg.tokenLenFn = token.EstimateToken
	}
	if len(cfg.separators) == 0 {
		cfg.separators = append([]string(nil), defaultParseBuildSeparators...)
	}
	if cfg.chunkSplitFn == nil {
		splitter, err := recursive.NewSplitter(context.Background(), &recursive.Config{
			ChunkSize:   cfg.maxNodeToken,
			OverlapSize: cfg.overlapToken,
			LenFunc:     cfg.tokenLenFn,
			KeepType:    recursive.KeepTypeEnd,
			Separators:  cfg.separators,
		})
		if err != nil {
			return nil, errors.Wrapf(errors.ErrInner, "create recursive splitter failed, err=%v", err)
		}
		cfg.chunkSplitFn = func(ctx context.Context, content string) ([]string, error) {
			docs, err := splitter.Transform(ctx, []*einoschema.Document{
				{ID: "parsebuild", Content: content},
			})
			if err != nil {
				return nil, errors.Wrapf(errors.ErrInner, "split content by recursive splitter failed, err=%v", err)
			}

			chunks := make([]string, 0, len(docs))
			for _, doc := range docs {
				chunks = append(chunks, doc.Content)
			}

			return chunks, nil
		}
	}

	return cfg, nil
}

// 通过解析markdown语法树的方式构建树结构
//
// 如果无法从content中解析出markdown语法树则返回错误
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

	parser := goldmark.DefaultParser()
	reader := goldtext.NewReader(content)
	markdoc := parser.Parse(reader)

	// 虚拟根节点
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

			parent.children = append(parent.children, newNode)
			pushStack(newNode) // 新的节点可能作为其他节点的父节点
			curParent = newNode
		default:
			text := extractMarkdownBlockText(node, content)
			if text != "" {
				curParent.contents = append(curParent.contents, text)
			}
		}
	}

	err = b.generateVRootTitle(ctx, vroot)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate vroot title failed, err=%v", err)
	}

	rootList, err := b.splitOversizedNode(ctx, vroot, buildOptions, true)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "split oversized markdown tree node failed, err=%v", err)
	}

	root, nodes := buildDocTreeFromMarkdownNode(rootList[0])

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

func (b *DocTreeBuilder) generateVRootTitle(ctx context.Context, vroot *markdownDocTreeNode) error {
	titles := collectMarkdownNodeTitles(vroot.children)
	if len(titles) == 0 {
		vroot.title = "vroot"
		return nil
	}
	titleContent := strings.Join(slices.Unique(titles), "\n")

	providerType := chat.Type(b.providerSelector(ctx))
	provider, err := b.gateway.GetProvider(providerType)
	if err != nil {
		return errors.Wrapf(errors.ErrInner, "get provider failed, err=%v", err)
	}

	model := b.modelSelector(ctx)
	llmOption := chat.BuildLLMModelOption(model)
	msg, err := prompts.SummarizePromptMessage(ctx, titleContent, "")
	if err != nil {
		return errors.Wrapf(errors.ErrInner, "render summarize prompt failed, err=%v", err)
	}

	genResp, err := provider.Generate(ctx, []*einoschema.Message{msg}, llmOption)
	if err != nil {
		return errors.Wrapf(errors.ErrInner, "generate vroot summary failed, err=%v", err)
	}
	summary := strings.TrimSpace(genResp.Content)
	if summary == "" {
		summary = "vroot"
	}
	vroot.title = summary

	return nil
}

func collectMarkdownNodeTitles(nodes []*markdownDocTreeNode) []string {
	titles := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if title := strings.TrimSpace(node.title); title != "" {
			titles = append(titles, title)
		}
		titles = append(titles, collectMarkdownNodeTitles(node.children)...)
	}
	return titles
}

func (b *DocTreeBuilder) splitOversizedNode(
	ctx context.Context,
	node *markdownDocTreeNode,
	opt *parseBuildOptions,
	isRoot bool,
) ([]*markdownDocTreeNode, error) {
	// 先处理子节点，再处理当前节点。
	children := make([]*markdownDocTreeNode, 0, len(node.children))
	for _, child := range node.children {
		newChildren, err := b.splitOversizedNode(ctx, child, opt, false)
		if err != nil {
			return nil, err
		}
		children = append(children, newChildren...)
	}
	node.children = children

	content := joinMarkdownNodeContent(node.contents)
	// 压缩空内容节点：叶子删除，非叶子上提子节点；根节点保留。
	if !isRoot && !hasNodeEmbeddingContent(node.title, content) {
		if len(node.children) == 0 {
			return nil, nil
		}
		return node.children, nil
	}
	if content == "" || !needSplitMarkdownNode(content, opt) {
		return []*markdownDocTreeNode{node}, nil
	}

	chunks, err := opt.chunkSplitFn(ctx, content)
	if err != nil {
		return nil, err
	}
	chunks = normalizeNodeContentChunks(chunks)
	if len(chunks) <= 1 {
		return []*markdownDocTreeNode{node}, nil
	}

	// 叶子节点横向分裂，分裂节点和原节点同父。
	if len(node.children) == 0 && !isRoot {
		siblings := make([]*markdownDocTreeNode, 0, len(chunks))
		for _, chunk := range chunks {
			siblings = append(siblings, &markdownDocTreeNode{
				title:        node.title,
				contents:     []string{chunk},
				headingLevel: node.headingLevel,
			})
		}
		return siblings, nil
	}

	// 非叶子节点：保留 title，content 下放为子节点（子节点不带 title）。
	chunkNodes := make([]*markdownDocTreeNode, 0, len(chunks))
	for _, chunk := range chunks {
		chunkNodes = append(chunkNodes, &markdownDocTreeNode{
			title:        "",
			contents:     []string{chunk},
			headingLevel: node.headingLevel + 1,
		})
	}
	node.contents = nil
	node.children = append(chunkNodes, node.children...)
	return []*markdownDocTreeNode{node}, nil
}

func needSplitMarkdownNode(content string, opt *parseBuildOptions) bool {
	return opt.tokenLenFn(content) > opt.maxNodeToken
}

func normalizeNodeContentChunks(chunks []string) []string {
	result := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		trimmed := strings.Trim(chunk, "\n\r")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func joinMarkdownNodeContent(contents []string) string {
	if len(contents) == 0 {
		return ""
	}
	joined := strings.Join(contents, "\n")
	if strings.TrimSpace(joined) == "" {
		return ""
	}
	return joined
}

func buildDocTreeFromMarkdownNode(
	root *markdownDocTreeNode,
) (*DocTreeNode, []*DocTreeNode) {
	type buildState struct {
		leafPos   int
		parentPos int
		nodes     []*DocTreeNode
	}

	state := &buildState{
		leafPos:   0,
		parentPos: -1,
		nodes:     make([]*DocTreeNode, 0),
	}

	var build func(node *markdownDocTreeNode) *DocTreeNode
	build = func(node *markdownDocTreeNode) *DocTreeNode {
		children := make([]*DocTreeNode, 0, len(node.children))
		derivedFrom := make([]string, 0)
		maxChildLevel := 0
		for _, child := range node.children {
			childNode := build(child)
			children = append(children, childNode)
			derivedFrom = append(derivedFrom, childNode.derivedFrom...)
			if childNode.level > maxChildLevel {
				maxChildLevel = childNode.level
			}
		}

		id := uuid.NewV4().String()
		node.id = id
		content := buildNodeEmbeddingContent(node.title, joinMarkdownNodeContent(node.contents))
		core := &vschema.SourceDoc{
			Id:       id,
			Content:  content,
			ChunkPos: -1,
		}

		if len(children) == 0 {
			derivedFrom = []string{id}
		} else {
			derivedFrom = slices.Unique(derivedFrom)
		}
		node.derivedFrom = derivedFrom

		level := 0
		pos := state.leafPos
		if len(children) == 0 {
			state.leafPos++
		} else {
			level = maxChildLevel + 1
			pos = state.parentPos
			state.parentPos--
		}

		docNode := &DocTreeNode{
			core:        core,
			level:       level,
			pos:         pos,
			children:    children,
			derivedFrom: derivedFrom,
		}
		core.ChunkPos = int32(pos)
		state.nodes = append(state.nodes, docNode)
		return docNode
	}

	return build(root), state.nodes
}

func buildNodeEmbeddingContent(title string, content string) string {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	switch {
	case title == "":
		return content
	case content == "":
		return title
	default:
		return title + "\n" + content
	}
}

func hasNodeEmbeddingContent(title string, content string) bool {
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
	batchSize, maxConcurrency := resolveParseBuildEmbedBatchSettings(len(texts), opt)
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

func resolveParseBuildEmbedBatchSettings(
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
