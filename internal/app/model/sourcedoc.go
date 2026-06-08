package model

import (
	vecschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const (
	SourceDocMetaDerivingPos = "_doc_deriving_pos"      // 派生节点的来源非派生节点pos bitmap
	SourceDocMetaLevel       = "_doc_tree_level"        // 节点在树中的层级
	SourceDocMetaChildrenPos = "_doc_node_children_pos" // 节点的子节点pos列表

	ChunkMetaPosStartKey     = "_doc_pos_rune_start"
	ChunkMetaPosEndKey       = "_doc_pos_rune_end"
	ChunkMetaPosByteStartKey = "_doc_pos_byte_start"
	ChunkMetaPosByteEndKey   = "_doc_pos_byte_end"
)

type SourceDocPosition struct {
	Start int
	End   int
}

func (p *SourceDocPosition) GetStart() int {
	if p == nil {
		return 0
	}

	return p.Start
}

func (p *SourceDocPosition) GetEnd() int {
	if p == nil {
		return 0
	}

	return p.End
}

type SourceDoc struct {
	Id         string
	NotebookId Id
	SourceId   Id
	Content    string
	Owner      string

	// 召回阶段分数 仅在召回阶段有值
	Score float32

	// 片段id
	ChunkPos int32

	// 派生自哪些非派生节点Id 仅在召回阶段值有效
	Derivation  []Id
	derivingPos string

	BytePos *SourceDocPosition
	RunePos *SourceDocPosition
}

func (s *SourceDoc) IsDerived() bool {
	return s.ChunkPos < 0
}

func (s *SourceDoc) DerivingPos() string {
	if s == nil {
		return ""
	}
	return s.derivingPos
}

func NewSourceDoc(doc *vecschema.SourceDoc) (*SourceDoc, error) {
	notebookId, err := uuid.ParseString(doc.NotebookId)
	if err != nil {
		return nil, err
	}
	sourceId, err := uuid.ParseString(doc.SourceId)
	if err != nil {
		return nil, err
	}

	sdc := &SourceDoc{
		Id:         doc.Id,
		NotebookId: notebookId,
		SourceId:   sourceId,
		Content:    doc.Content,
		Score:      doc.Score,
		Owner:      doc.Owner,
		ChunkPos:   doc.ChunkPos,
	}

	// 注意 Derivation 字段在外部设置 因为derivation需要额外查询Id映射
	derivingPos, ok := doc.GetStringMeta(SourceDocMetaDerivingPos)
	if ok {
		sdc.derivingPos = derivingPos
	}

	byteStart, ok1 := doc.GetInt64Meta(ChunkMetaPosByteStartKey)
	byteEnd, ok2 := doc.GetInt64Meta(ChunkMetaPosByteEndKey)
	if ok1 && ok2 {
		sdc.BytePos = &SourceDocPosition{
			Start: int(byteStart),
			End:   int(byteEnd),
		}
	}
	runeStart, ok3 := doc.GetInt64Meta(ChunkMetaPosStartKey)
	runeEnd, ok4 := doc.GetInt64Meta(ChunkMetaPosEndKey)
	if ok3 && ok4 {
		sdc.RunePos = &SourceDocPosition{
			Start: int(runeStart),
			End:   int(runeEnd),
		}
	}

	return sdc, nil
}
