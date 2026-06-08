package source

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/indices"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	vschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/bitmap"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRecoverDocTreeNonDerivedNonLeafByChildrenMeta(t *testing.T) {
	Convey("recoverDocTree 应支持非派生非叶子节点使用正 pos", t, func() {
		leafDoc := &vschema.SourceDoc{
			Id:         "leaf-doc",
			NotebookId: "nb",
			SourceId:   "src",
			Owner:      "owner",
			Content:    "leaf",
			ChunkPos:   0,
		}
		leafDoc.PutMeta(model.SourceDocMetaLevel, int64(0))

		parentBitmap := bitmap.New(2)
		parentBitmap.Set(1) // parent 自己是非派生节点，derivation 指向自己
		parentDoc := &vschema.SourceDoc{
			Id:         "parent-doc",
			NotebookId: "nb",
			SourceId:   "src",
			Owner:      "owner",
			Content:    "parent",
			ChunkPos:   1,
		}
		parentDoc.PutMeta(model.SourceDocMetaLevel, int64(1))
		parentDoc.PutMeta(model.SourceDocMetaChildrenPos, []int{0})
		parentDoc.PutMeta(model.SourceDocMetaDerivingPos, parentBitmap.String())

		rootBitmap := bitmap.New(2)
		rootBitmap.Set(1) // root 派生自 parent 这个非派生节点
		rootDoc := &vschema.SourceDoc{
			Id:         "root-doc",
			NotebookId: "nb",
			SourceId:   "src",
			Owner:      "owner",
			Content:    "root",
			ChunkPos:   -1,
		}
		rootDoc.PutMeta(model.SourceDocMetaLevel, int64(2))
		rootDoc.PutMeta(model.SourceDocMetaChildrenPos, []int{1})
		rootDoc.PutMeta(model.SourceDocMetaDerivingPos, rootBitmap.String())

		tree, err := recoverDocTree(context.Background(), []*vschema.SourceDoc{
			rootDoc, parentDoc, leafDoc,
		})
		So(err, ShouldBeNil)
		So(tree, ShouldNotBeNil)
		So(tree.Root(), ShouldNotBeNil)
		So(tree.Root().Derivation(), ShouldResemble, []string{"parent-doc"})

		var (
			parentNode *indices.DocTreeNode
			leafNode   *indices.DocTreeNode
		)
		for _, node := range tree.Nodes() {
			if node == nil || node.Core() == nil {
				continue
			}
			switch node.Core().Id {
			case "parent-doc":
				parentNode = node
			case "leaf-doc":
				leafNode = node
			}
		}

		So(parentNode, ShouldNotBeNil)
		So(parentNode.IsLeaf(), ShouldBeFalse)
		So(parentNode.Derivation(), ShouldResemble, []string{"parent-doc"})
		So(len(parentNode.Children()), ShouldEqual, 1)
		So(parentNode.Children()[0].Core().Id, ShouldEqual, "leaf-doc")

		So(leafNode, ShouldNotBeNil)
		So(leafNode.IsLeaf(), ShouldBeTrue)
		So(leafNode.Derivation(), ShouldResemble, []string{"leaf-doc"})
	})
}
