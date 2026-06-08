package source

import (
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/indices"
)

// ParsedSourceDocTree is the API-facing compact tree view.
// It intentionally excludes internal fields such as derivation
type ParsedSourceDocTree struct {
	Root   *ParsedSourceDocTreeNode `json:"root,omitempty"`
	Height int                      `json:"height"`
}

type ParsedSourceDocTreeNode struct {
	Id       string                     `json:"id"`
	Content  string                     `json:"content"`
	Level    int                        `json:"level"`
	Pos      int                        `json:"pos"`
	IsLeaf   bool                       `json:"is_leaf"`
	Children []*ParsedSourceDocTreeNode `json:"children,omitempty"`
}

func buildParsedSourceDocTree(tree *indices.DocTree) *ParsedSourceDocTree {
	if tree == nil {
		return &ParsedSourceDocTree{}
	}

	return &ParsedSourceDocTree{
		Root:   buildParsedSourceDocTreeNode(tree.Root()),
		Height: tree.Height(),
	}
}

func buildParsedSourceDocTreeNode(node *indices.DocTreeNode) *ParsedSourceDocTreeNode {
	if node == nil {
		return nil
	}

	core := node.Core()
	parsed := &ParsedSourceDocTreeNode{
		Level:  node.Level(),
		Pos:    node.Pos(),
		IsLeaf: node.IsLeaf(),
	}
	if core != nil {
		parsed.Id = core.Id
		parsed.Content = core.Content
	}

	children := node.Children()
	if len(children) == 0 {
		return parsed
	}

	parsed.Children = make([]*ParsedSourceDocTreeNode, 0, len(children))
	for _, child := range children {
		parsed.Children = append(parsed.Children, buildParsedSourceDocTreeNode(child))
	}

	return parsed
}
