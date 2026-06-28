package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListNotebooksHandler struct {
	notebookRepo notebook.Repository
}

func NewListNotebooksHandler(notebookRepo notebook.Repository) *ListNotebooksHandler {
	return &ListNotebooksHandler{
		notebookRepo: notebookRepo,
	}
}

type SortBy int

const (
	SortByCreateTime SortBy = 0
	SortByLastActive SortBy = 1
)

type ListNotebooksHandleQuery struct {
	OwnerId string
	Limit   int
	Offset  int
	SortBy  SortBy
}

func (q *ListNotebooksHandleQuery) ToSpec() *notebook.ListSpec {
	order := notebook.ListSpecOrderCreateTime
	if q.SortBy == SortByLastActive {
		order = notebook.ListSpecOrderUpdateTime
	}
	return &notebook.ListSpec{
		Offset: q.Offset,
		Limit:  q.Limit,
		Order:  order,
	}
}

type ListNotebooksHandleResult struct {
	Notebooks []*notebook.Notebook
	Limit     int
	Offset    int
	HasMore   bool
}

func (h *ListNotebooksHandler) Handle(
	ctx context.Context,
	query *ListNotebooksHandleQuery,
) (*ListNotebooksHandleResult, error) {
	spec := query.ToSpec() // TODO query one more to check has more
	notebooks, err := h.notebookRepo.ListByOwner(ctx, query.OwnerId, spec)
	if err != nil {
		return nil, errors.WithMessage(err, "list notebooks failed")
	}

	return &ListNotebooksHandleResult{
		Notebooks: notebooks,
		Limit:     query.Limit,
		Offset:    query.Offset,
		HasMore:   len(notebooks) > query.Limit, // TODO query one more
	}, nil
}
