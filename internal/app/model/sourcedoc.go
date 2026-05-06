package model

import (
	vecschema "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type SourceDoc struct {
	Id         string
	NotebookId Id
	SourceId   Id
	Content    string
	Score      float32
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

	return &SourceDoc{
		Id:         doc.Id,
		NotebookId: notebookId,
		SourceId:   sourceId,
		Content:    doc.Content,
		Score:      doc.Score,
	}, nil
}
