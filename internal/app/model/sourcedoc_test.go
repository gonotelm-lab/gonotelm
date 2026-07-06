package model

import (
	"testing"

	vecschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"
)

func TestNewSourceDocTreeMeta(t *testing.T) {
	doc := &vecschema.SourceDoc{
		Id:         "doc-1",
		NotebookId: "11111111-1111-1111-1111-111111111111",
		SourceId:   "22222222-2222-2222-2222-222222222222",
		Content:    "content",
		Owner:      "owner",
		ChunkPos:   7,
	}
	doc.PutMeta(SourceDocMetaParentPos, 3)
	doc.PutMeta(SourceDocMetaChildrenPos, []int{8, 9})

	sourceDoc, err := NewSourceDoc(doc)
	if err != nil {
		t.Fatalf("new source doc failed: %v", err)
	}
	if sourceDoc.TreeMeta == nil {
		t.Fatal("tree meta should not be nil")
	}

	parentPos, ok := sourceDoc.TreeMeta.ParentPos()
	if !ok || parentPos != 3 {
		t.Fatalf("unexpected parent pos: ok=%v, pos=%d", ok, parentPos)
	}

	childrenPos := sourceDoc.TreeMeta.ChildrenPos()
	if len(childrenPos) != 2 || childrenPos[0] != 8 || childrenPos[1] != 9 {
		t.Fatalf("unexpected children pos: %v", childrenPos)
	}

	// ChildrenPos 返回副本，避免外部修改污染内部状态。
	childrenPos[0] = 999
	again := sourceDoc.TreeMeta.ChildrenPos()
	if len(again) != 2 || again[0] != 8 {
		t.Fatalf("children pos should be immutable copy: %v", again)
	}
}

func TestNewSourceDocTreeMetaNoParent(t *testing.T) {
	doc := &vecschema.SourceDoc{
		Id:         "doc-2",
		NotebookId: "11111111-1111-1111-1111-111111111111",
		SourceId:   "22222222-2222-2222-2222-222222222222",
		Content:    "content",
		Owner:      "owner",
		ChunkPos:   11,
	}
	doc.PutMeta(SourceDocMetaChildrenPos, []int{12})

	sourceDoc, err := NewSourceDoc(doc)
	if err != nil {
		t.Fatalf("new source doc failed: %v", err)
	}
	if sourceDoc.TreeMeta == nil {
		t.Fatal("tree meta should not be nil when children pos exists")
	}
	if _, ok := sourceDoc.TreeMeta.ParentPos(); ok {
		t.Fatal("parent pos should be absent")
	}
}
