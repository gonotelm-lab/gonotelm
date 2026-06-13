package studio

import (
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

// must embed in task params
type commonTaskParams struct {
	NotebookId uuid.UUID   `json:"notebook_id"`
	SourceIds  []uuid.UUID `json:"source_ids"`
}

func (p *commonTaskParams) getNotebookId() uuid.UUID {
	return p.NotebookId
}

func (p *commonTaskParams) getSourceIds() []uuid.UUID {
	return p.SourceIds
}

type iCommonTaskParams interface {
	getNotebookId() uuid.UUID
	getSourceIds() []uuid.UUID
}

var _ iCommonTaskParams = &commonTaskParams{}

