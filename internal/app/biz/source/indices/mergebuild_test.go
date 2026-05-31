package indices

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	vschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMergeBuildDerivedFromSemanticWithNonDerivedNodes(t *testing.T) {
	Convey("MergeBuild 派生节点应聚合非派生节点 derivedFrom", t, func() {
		mockLLM := &parseBuildMockLLM{response: "merge-root-summary"}
		mockEmbedder := &parseBuildMockEmbedder{}
		mockGateway := newParseBuildMockGateway(chat.Openai, mockLLM)
		builder := NewDocTreeBuilder(
			mockEmbedder,
			mockGateway,
			func(_ context.Context) string { return string(chat.Openai) },
			func(_ context.Context) string { return "mock-model" },
		)

		nodeA := NewDocTreeNode(&vschema.SourceDoc{
			Id:         "node-a",
			NotebookId: "nb",
			SourceId:   "src",
			Owner:      "owner",
			Content:    "content-a",
			Embedding:  []float32{0.1},
			ChunkPos:   0,
		}, 0, 0, nil, []string{"node-a"})
		nodeB := NewDocTreeNode(&vschema.SourceDoc{
			Id:         "node-b",
			NotebookId: "nb",
			SourceId:   "src",
			Owner:      "owner",
			Content:    "content-b",
			Embedding:  []float32{0.2},
			ChunkPos:   1,
		}, 0, 1, nil, []string{"node-b"})

		tree, err := builder.MergeBuild(context.Background(), []*DocTreeNode{nodeA, nodeB})
		So(err, ShouldBeNil)
		So(tree, ShouldNotBeNil)
		So(tree.Root(), ShouldNotBeNil)
		So(tree.Root().IsLeaf(), ShouldBeFalse)
		So(tree.Root().DerivedFrom(), ShouldResemble, []string{"node-a", "node-b"})
	})
}
