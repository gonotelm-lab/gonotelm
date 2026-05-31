package indices

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	vschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"

	einoembed "github.com/cloudwego/eino/components/embedding"
)

type DocTreeNode struct {
	core  *vschema.SourceDoc
	level int // 0-based, the deepest level is 0

	// 表示文档在来源中所处的位置（和 core.ChunkPos 一致）。
	// 当前分配策略下通常 pos<0 为派生节点、pos>=0 为非派生节点（非派生节点表示文本块内容来自原始来源）。
	pos      int
	children []*DocTreeNode

	// 衍生自哪些非派生节点
	// 如果是非派生节点 这个字段就是自己的id
	derivedFrom []string

	// 只有使用ParseBuild时节点才有parse的metadata
	parseMetadata *parseMetadata
}

func NewDocTreeNode(
	core *vschema.SourceDoc,
	level int,
	pos int,
	children []*DocTreeNode,
	derivedFrom []string,
) *DocTreeNode {
	return &DocTreeNode{
		core:        core,
		level:       level,
		pos:         pos,
		children:    children,
		derivedFrom: derivedFrom,
	}
}

func (n *DocTreeNode) Core() *vschema.SourceDoc {
	if n == nil {
		return nil
	}
	return n.core
}

func (n *DocTreeNode) Level() int {
	if n == nil {
		return 0
	}
	return n.level
}

func (n *DocTreeNode) Pos() int {
	if n == nil {
		return -1
	}
	return n.pos
}

func (n *DocTreeNode) IsLeaf() bool {
	if n == nil {
		return false
	}
	return len(n.children) == 0
}

func (n *DocTreeNode) Children() []*DocTreeNode {
	if n == nil {
		return nil
	}
	return n.children
}

func (n *DocTreeNode) DerivedFrom() []string {
	if n == nil {
		return nil
	}
	return n.derivedFrom
}

func (n *DocTreeNode) ParseMetadata() *parseMetadata {
	if n == nil {
		return nil
	}

	return n.parseMetadata
}

type (
	LLMProviderSelector func(ctx context.Context) string
	LLMModelSelector    func(ctx context.Context) string
)

type DocTreeBuilder struct {
	embedder einoembed.Embedder
	gateway  *gateway.Gateway

	providerSelector LLMProviderSelector
	modelSelector    LLMModelSelector
}

func NewDocTreeBuilder(
	embedder einoembed.Embedder,
	gateway *gateway.Gateway,
	providerSelector LLMProviderSelector,
	modelSelector LLMModelSelector,
) *DocTreeBuilder {
	return &DocTreeBuilder{
		embedder:         embedder,
		gateway:          gateway,
		providerSelector: providerSelector,
		modelSelector:    modelSelector,
	}
}

type DocTree struct {
	root   *DocTreeNode
	nodes  []*DocTreeNode // all nodes in the tree including root
	height int
}

func (t *DocTree) Nodes() []*DocTreeNode {
	if t == nil {
		return nil
	}
	return t.nodes
}

func (t *DocTree) SetNodes(nodes []*DocTreeNode) {
	if t == nil {
		return
	}

	t.nodes = nodes
}

func (t *DocTree) SetRoot(root *DocTreeNode) {
	if t == nil {
		return
	}

	t.root = root
}

func (t *DocTree) SetHeight(height int) {
	if t == nil {
		return
	}

	t.height = height
}

func (t *DocTree) Root() *DocTreeNode {
	if t == nil {
		return nil
	}
	return t.root
}

func (t *DocTree) Height() int {
	if t == nil {
		return 0
	}
	return t.height
}
