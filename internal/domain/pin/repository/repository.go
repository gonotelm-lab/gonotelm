package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
)

type Repository interface {
	Save(ctx context.Context, pin *pinentity.UserNotebookPin) error
	// Find 不存在时返回 ErrPinNotFound。
	Find(ctx context.Context, userId valobj.Uid, notebookId valobj.Id) (*pinentity.UserNotebookPin, error)
	ListByUserId(ctx context.Context, userId valobj.Uid) ([]*pinentity.UserNotebookPin, error)
	CountByUserId(ctx context.Context, userId valobj.Uid) (int64, error)
	// Delete / DeleteByNotebookId 幂等。
	Delete(ctx context.Context, userId valobj.Uid, notebookId valobj.Id) error
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}
