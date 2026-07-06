package indices

import (
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/textgen/summarizer"
	vschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"

	einoembed "github.com/cloudwego/eino/components/embedding"
)

type DocTreeNode struct {
	core  *vschema.SourceDoc
	level int // 0-based, the deepest level is 0

	// 表示文档在来源中所处的位置（和 core.ChunkPos 一致）。
	// 当前分配策略下通常 pos<0 为派生节点、pos>=0 为非派生节点（非派生节点表示文本块内容来自原始来源）。
	pos    int
	parent *DocTreeNode

	children []*DocTreeNode

	// 衍生自哪些非派生节点
	// 如果是非派生节点 这个字段就是自己的id
	derivation []string

	// 只有使用ParseBuild时节点才有parse的metadata
	parseMetadata *parseMetadata
}

func NewDocTreeNode(
	core *vschema.SourceDoc,
	level int,
	pos int,
	children []*DocTreeNode,
	derivation []string,
) *DocTreeNode {
	node := &DocTreeNode{
		core:       core,
		level:      level,
		pos:        pos,
		children:   children,
		derivation: derivation,
	}
	bindParentForChildren(node.children, node)
	return node
}

// bindParentForChildren 只负责连接当前层的 parent -> children 关系，
// 递归层级由各构建流程按需触发，避免在基础结构层引入隐式深遍历。
func bindParentForChildren(children []*DocTreeNode, parent *DocTreeNode) {
	for _, child := range children {
		if child == nil {
			continue
		}
		child.parent = parent
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

func (n *DocTreeNode) Parent() *DocTreeNode {
	if n == nil {
		return nil
	}
	return n.parent
}

func (n *DocTreeNode) Derivation() []string {
	if n == nil {
		return nil
	}
	return n.derivation
}

func (n *DocTreeNode) ParseMetadata() *parseMetadata {
	if n == nil {
		return nil
	}

	return n.parseMetadata
}

type DocTreeBuilder struct {
	embedder   einoembed.Embedder
	summarizer summarizer.Summarizer
}

func NewDocTreeBuilder(
	embedder einoembed.Embedder,
	summarizer summarizer.Summarizer,
) *DocTreeBuilder {
	return &DocTreeBuilder{
		embedder:   embedder,
		summarizer: summarizer,
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
