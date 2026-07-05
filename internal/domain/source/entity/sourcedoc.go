package entity

import (
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const (
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

// SourceDoc 表示一个文档片段，用于召回阶段和索引阶段
//
// 该对象实体是Source对象的分块
type SourceDoc struct {
	Id         valobj.Id
	NotebookId valobj.Id
	SourceId   valobj.Id
	Content    string
	Owner      string

	// 召回阶段分数 仅在召回阶段有值
	Score float32

	// 片段id
	ChunkPos int

	BytePos *SourceDocPosition
	RunePos *SourceDocPosition
}

func NewSourceDoc(
	sourceId, notebookId valobj.Id,
	owner string,
	chunkPos int,
	doc *einoschema.Document,
) (*SourceDoc, error) {
	byteStart, _ := doc.MetaData[ChunkMetaPosByteStartKey].(int)
	byteEnd, _ := doc.MetaData[ChunkMetaPosByteEndKey].(int)
	runeStart, _ := doc.MetaData[ChunkMetaPosStartKey].(int)
	runeEnd, _ := doc.MetaData[ChunkMetaPosEndKey].(int)

	return &SourceDoc{
		Id:         valobj.NewUnOrderedId(),
		NotebookId: notebookId,
		SourceId:   sourceId,
		Content:    doc.Content,
		Owner:      owner,
		ChunkPos:   chunkPos,
		BytePos: &SourceDocPosition{
			Start: byteStart,
			End:   byteEnd,
		},
		RunePos: &SourceDocPosition{
			Start: runeStart,
			End:   runeEnd,
		},
	}, nil
}
