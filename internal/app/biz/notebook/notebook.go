package notebook

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const (
	untitledNotebookName = "Untitled notebook"
)

var ErrNotebookNotFound = errors.New("notebook not found")

type Biz struct {
	notebookStore dal.NotebookStore
}

func New(notebookStore dal.NotebookStore) *Biz {
	return &Biz{notebookStore: notebookStore}
}

func (b *Biz) GetNotebook(ctx context.Context, id uuid.UUID) (*model.Notebook, error) {
	notebook, err := b.notebookStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, ErrNotebookNotFound
		}
		return nil, errors.WithMessage(err, "get notebook failed")
	}

	return model.NewNotebookFrom(notebook), nil
}

type ListNotebooksQuery struct {
	Limit   int
	Offset  int
	OwnerId string
}

type ListNotebooksResult struct {
	Notebooks []*model.Notebook
	HasMore   bool
}

func (b *Biz) ListNotebooks(
	ctx context.Context,
	query *ListNotebooksQuery,
) (*ListNotebooksResult, error) {
	fetchLimit := query.Limit + 1
	rows, err := b.notebookStore.ListByOwnerId(
		ctx,
		query.OwnerId,
		fetchLimit,
		query.Offset,
		0,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "store list notebooks failed")
	}

	hasMore := len(rows) > query.Limit
	if hasMore {
		rows = rows[:query.Limit]
	}

	notebooks := make([]*model.Notebook, 0, len(rows))
	for i := range rows {
		row := rows[i]
		notebooks = append(notebooks, model.NewNotebookFrom(row))
	}

	return &ListNotebooksResult{
		Notebooks: notebooks,
		HasMore:   hasMore,
	}, nil
}

type CreateNotebookCommand struct {
	OwnerId string
	Name    string
	Desc    string
}

func (b *Biz) CreateNotebook(ctx context.Context, cmd *CreateNotebookCommand) (*model.Notebook, error) {
	notebookId := uuid.NewV7()

	if cmd.Name == "" {
		cmd.Name = untitledNotebookName
	}
	notebook := &model.Notebook{
		Id:          notebookId,
		Name:        cmd.Name,
		Description: cmd.Desc,
		OwnerId:     cmd.OwnerId,
		UpdatedAt:   time.Now().UnixMilli(),
	}

	err := b.notebookStore.Create(ctx, notebook.To())
	if err != nil {
		return nil, errors.WithMessage(err, "store create notebook failed")
	}

	return notebook, nil
}

func (b *Biz) UpdateNotebookName(ctx context.Context, id uuid.UUID, name string) error {
	err := b.notebookStore.UpdateName(ctx, id, name)
	if err != nil {
		return errors.WithMessage(err, "store update notebook name failed")
	}

	return nil
}

func (b *Biz) UpdateNotebookDesc(ctx context.Context, id uuid.UUID, desc string) error {
	err := b.notebookStore.UpdateDesc(ctx, id, desc)
	if err != nil {
		return errors.WithMessage(err, "store update notebook desc failed")
	}

	return nil
}
