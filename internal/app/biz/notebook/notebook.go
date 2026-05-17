package notebook

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
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
	SortBy  int // 0-create_time, 1-updated_at
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
		query.SortBy,
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
	OwnerId     string
	Name        string // optional
	Description string // optional
}

func (b *Biz) CreateNotebook(ctx context.Context, cmd *CreateNotebookCommand) (*model.Notebook, error) {
	notebookId := uuid.NewV7()
	notebook := &model.Notebook{
		Id:          notebookId,
		Name:        cmd.Name,
		Description: cmd.Description,
		OwnerId:     cmd.OwnerId,
		UpdatedAt:   time.Now().UnixMilli(),
	}

	err := b.notebookStore.Create(ctx, notebook.To())
	if err != nil {
		return nil, errors.WithMessage(err, "store create notebook failed")
	}

	return notebook, nil
}

func (b *Biz) UpdateNotebookName(
	ctx context.Context,
	id uuid.UUID,
	name string,
) error {
	err := b.notebookStore.UpdateName(ctx, &schema.NotebookUpdateNameParams{
		Id:        id,
		Name:      name,
		UpdatedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessage(err, "store update notebook name failed")
	}

	return nil
}

type UpdateNotebookDescriptionCommand struct {
	Id             uuid.UUID
	Description    string
	SkipIfNonEmpty bool
}

func (b *Biz) UpdateNotebookDescription(
	ctx context.Context,
	cmd *UpdateNotebookDescriptionCommand,
) error {
	err := b.notebookStore.UpdateDescription(ctx, &schema.NotebookUpdateDescriptionParams{
		Id:             cmd.Id,
		Description:    cmd.Description,
		SkipIfNonEmpty: cmd.SkipIfNonEmpty,
		UpdatedAt:      time.Now().UnixMilli(),
	})
	if err != nil {
		return errors.WithMessage(err, "store update notebook description failed")
	}

	return nil
}

type FillNotebookMetaCommand struct {
	Id          uuid.UUID
	Name        string
	Description string
}

// notebook meta = notebook name + description
func (b *Biz) FillNotebookMeta(
	ctx context.Context,
	cmd *FillNotebookMetaCommand,
) error {
	err := b.notebookStore.FillNameAndDescriptionIfEmpty(ctx,
		&schema.NotebookFillNameAndDescriptionParams{
			Id:          cmd.Id,
			Name:        cmd.Name,
			Description: cmd.Description,
			UpdatedAt:   time.Now().UnixMilli(),
		})
	if err != nil {
		return errors.WithMessage(err, "store fill notebook meta failed")
	}

	return nil
}
