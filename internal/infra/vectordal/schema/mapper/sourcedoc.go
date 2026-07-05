package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
)

func SchemaToSourceDoc(schema *schema.SourceDoc) (*entity.SourceDoc, error) {
	notebookId, err := valobj.NewIdFromString(schema.NotebookId)
	if err != nil {
		return nil, err
	}
	sourceId, err := valobj.NewIdFromString(schema.SourceId)
	if err != nil {
		return nil, err
	}

	id, err := valobj.NewIdFromString(schema.Id)
	if err != nil {
		return nil, err
	}

	domainDoc := &entity.SourceDoc{
		Id:         id,
		NotebookId: notebookId,
		SourceId:   sourceId,
		Content:    schema.Content,
		Owner:      schema.Owner,
		ChunkPos:   int(schema.ChunkPos),
		Score:      schema.Score,
		BytePos:    &entity.SourceDocPosition{},
		RunePos:    &entity.SourceDocPosition{},
	}

	if byteStart, ok := schema.GetInt64Meta(entity.ChunkMetaPosByteStartKey); ok {
		domainDoc.BytePos.Start = int(byteStart)
	}
	if byteEnd, ok := schema.GetInt64Meta(entity.ChunkMetaPosByteEndKey); ok {
		domainDoc.BytePos.End = int(byteEnd)
	}
	if runeStart, ok := schema.GetInt64Meta(entity.ChunkMetaPosStartKey); ok {
		domainDoc.RunePos.Start = int(runeStart)
	}
	if runeEnd, ok := schema.GetInt64Meta(entity.ChunkMetaPosEndKey); ok {
		domainDoc.RunePos.End = int(runeEnd)
	}

	return domainDoc, nil
}

func SchemasToSourceDocs(schemas []*schema.SourceDoc) ([]*entity.SourceDoc, error) {
	domainDocs := make([]*entity.SourceDoc, 0, len(schemas))
	for _, schema := range schemas {
		domainDoc, err := SchemaToSourceDoc(schema)
		if err != nil {
			return nil, err
		}

		domainDocs = append(domainDocs, domainDoc)
	}

	return domainDocs, nil
}

// 注意没有embedding和score
func SourceDocToSchema(doc *entity.SourceDoc) *schema.SourceDoc {
	schemaDoc := &schema.SourceDoc{
		Id:         doc.Id.String(),
		NotebookId: doc.NotebookId.String(),
		SourceId:   doc.SourceId.String(),
		Content:    doc.Content,
		Owner:      doc.Owner,
		ChunkPos:   int32(doc.ChunkPos),
	}

	// all are int
	if doc.BytePos != nil {
		schemaDoc.PutMeta(entity.ChunkMetaPosByteStartKey, doc.BytePos.Start)
		schemaDoc.PutMeta(entity.ChunkMetaPosByteEndKey, doc.BytePos.End)
	}
	if doc.RunePos != nil {
		schemaDoc.PutMeta(entity.ChunkMetaPosStartKey, doc.RunePos.Start)
		schemaDoc.PutMeta(entity.ChunkMetaPosEndKey, doc.RunePos.End)
	}

	return schemaDoc
}
