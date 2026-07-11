package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
)

type SourceDocRepository interface {
	FindById(ctx context.Context, notebookId valobj.Id, sourceId valobj.Id, docId valobj.Id) (*entity.SourceDoc, error)

	// if sourceId == "" then find condition will be where notebookId = ? and docId in (?)
	// if sourceId != "" then find condition will be where notebookId = ? and sourceId = ? and docId in (?)
	BatchFind(ctx context.Context, notebookId valobj.Id, sourceId valobj.Id, docIds []valobj.Id) ([]*entity.SourceDoc, error)

	BatchSave(ctx context.Context, docs []*entity.SourceDoc) error
	BatchDeleteBySourceId(ctx context.Context, notebookId valobj.Id, sourceId []valobj.Id) error
	Query(ctx context.Context, query *SourceDocQueryParams) ([]*entity.SourceDoc, error)
}

type SourceDocQueryParams struct {
	// Target notebook id
	NotebookId valobj.Id

	// Target source ids
	SourceId []valobj.Id

	// Target queried text
	Target string

	// top-k returned docs
	Limit int
}
