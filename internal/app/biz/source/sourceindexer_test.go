package source

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/indices"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"
	vschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/bitmap"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
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
		So(tree.Root().Parent(), ShouldBeNil)

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
		So(parentNode.Parent(), ShouldEqual, tree.Root())
		So(len(parentNode.Children()), ShouldEqual, 1)
		So(parentNode.Children()[0].Core().Id, ShouldEqual, "leaf-doc")

		So(leafNode, ShouldNotBeNil)
		So(leafNode.IsLeaf(), ShouldBeTrue)
		So(leafNode.Derivation(), ShouldResemble, []string{"leaf-doc"})
		So(leafNode.Parent(), ShouldEqual, parentNode)
	})
}

type sourceDocStoreStub struct {
	docsBySourcePos map[string]map[int32]*vschema.SourceDoc
}

func (s *sourceDocStoreStub) BatchInsert(_ context.Context, _ []*schema.SourceDoc) error {
	return nil
}

func (s *sourceDocStoreStub) BatchDelete(_ context.Context, _ *schema.SourceDocBatchDeleteParams) error {
	return nil
}

func (s *sourceDocStoreStub) Get(_ context.Context, _ *schema.SourceDocGetParams) (*schema.SourceDoc, error) {
	return nil, nil
}

func (s *sourceDocStoreStub) BatchGet(
	_ context.Context,
	_ *schema.SourceDocBatchGetParams,
) ([]*schema.SourceDoc, error) {
	return nil, nil
}

func (s *sourceDocStoreStub) Query(_ context.Context, _ *schema.SourceDocQueryParams) ([]*schema.SourceDoc, error) {
	return nil, nil
}

func (s *sourceDocStoreStub) List(_ context.Context, _ *schema.SourceDocListParams) ([]*schema.SourceDoc, error) {
	return nil, nil
}

func (s *sourceDocStoreStub) ListByChunkPos(
	_ context.Context,
	params *schema.SourceDocListByChunkPosParams,
) ([]*schema.SourceDoc, error) {
	if params == nil {
		return nil, nil
	}
	docsByPos := s.docsBySourcePos[params.SourceId]
	out := make([]*schema.SourceDoc, 0, len(params.ChunkPoses))
	for _, pos := range params.ChunkPoses {
		if doc, ok := docsByPos[pos]; ok && doc != nil {
			out = append(out, doc)
		}
	}
	return out, nil
}

func TestPopulateSourceDocsFillTreeMetaByPos(t *testing.T) {
	Convey("PopulateSourceDocs 应通过 pos 回填 ParentId 与 Children", t, func() {
		notebookID := "11111111-1111-1111-1111-111111111111"
		sourceID := uuid.MustParseString("22222222-2222-2222-2222-222222222222").String()
		parentID := "33333333-3333-3333-3333-333333333333"
		childID := "44444444-4444-4444-4444-444444444444"

		parentVecDoc := &vschema.SourceDoc{
			Id:         parentID,
			NotebookId: notebookID,
			SourceId:   sourceID,
			ChunkPos:   1,
		}
		parentVecDoc.PutMeta(model.SourceDocMetaChildrenPos, []int{2})
		childVecDoc := &vschema.SourceDoc{
			Id:         childID,
			NotebookId: notebookID,
			SourceId:   sourceID,
			ChunkPos:   2,
		}
		childVecDoc.PutMeta(model.SourceDocMetaParentPos, 1)

		parentDoc, err := model.NewSourceDoc(parentVecDoc)
		So(err, ShouldBeNil)
		childDoc, err := model.NewSourceDoc(childVecDoc)
		So(err, ShouldBeNil)
		So(parentDoc.TreeMeta, ShouldNotBeNil)
		So(childDoc.TreeMeta, ShouldNotBeNil)

		store := &sourceDocStoreStub{
			docsBySourcePos: map[string]map[int32]*vschema.SourceDoc{
				sourceID: {
					1: parentVecDoc,
					2: childVecDoc,
				},
			},
		}
		biz := &Biz{
			sourceDocStore: store,
		}

		err = biz.PopulateSourceDocs(
			context.Background(),
			uuid.MustParseString(notebookID),
			[]*model.SourceDoc{parentDoc, childDoc},
		)
		So(err, ShouldBeNil)

		expectedParentID := uuid.MustParseString(parentID)
		expectedChildID := uuid.MustParseString(childID)
		So(childDoc.TreeMeta.ParentId.EqualsTo(expectedParentID), ShouldBeTrue)
		So(parentDoc.TreeMeta.Children, ShouldHaveLength, 1)
		So(parentDoc.TreeMeta.Children[0].EqualsTo(expectedChildID), ShouldBeTrue)
	})
}
