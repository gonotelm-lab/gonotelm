package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListNotebooksHandler struct {
	notebookRepo notebookrepo.Repository
}

func NewListNotebooksHandler(notebookRepo notebookrepo.Repository) *ListNotebooksHandler {
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

func (q *ListNotebooksHandleQuery) ToSpec() *notebookrepo.ListSpec {
	order := notebookrepo.ListSpecOrderCreateTime
	if q.SortBy == SortByLastActive {
		order = notebookrepo.ListSpecOrderUpdateTime
	}
	return &notebookrepo.ListSpec{
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
	spec := query.ToSpec()
	spec.Limit += 1
	notebooks, err := h.notebookRepo.ListByOwner(ctx, query.OwnerId, spec)
	if err != nil {
		return nil, errors.WithMessage(err, "list notebooks failed")
	}

	return &ListNotebooksHandleResult{
		Notebooks: notebooks,
		Limit:     query.Limit,
		Offset:    query.Offset,
		HasMore:   len(notebooks) > query.Limit,
	}, nil
}
