package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
)

type Repository interface {
	Save(ctx context.Context, chat *entity.Chat) error
	FindById(ctx context.Context, id valobj.Id) (*entity.Chat, error)
	FindByNotebookIdAndOwnerId(ctx context.Context, notebookId valobj.Id, ownerId string) (*entity.Chat, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id) ([]*entity.Chat, error)
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}

type ListSpec struct {
	Offset int
	Limit  int
}
