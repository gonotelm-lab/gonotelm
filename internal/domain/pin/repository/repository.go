package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
)

type Repository interface {
	Save(ctx context.Context, pin *pinentity.UserNotebookPin) error
	// Find 不存在时返回 ErrNoRecord / ErrPinNotFound。
	Find(ctx context.Context, userId valobj.Uid, notebookId valobj.Id) (*pinentity.UserNotebookPin, error)
	// ListByUserId 按 pinned_at DESC 返回该用户全部置顶。
	ListByUserId(ctx context.Context, userId valobj.Uid) ([]*pinentity.UserNotebookPin, error)
	// CountByUserAndNotebookIds 统计 user 在给定 notebook id 集合中的置顶数（用于 scope 上限）。
	CountByUserAndNotebookIds(ctx context.Context, userId valobj.Uid, notebookIds []valobj.Id) (int64, error)
	// Delete 幂等：无行不报错。
	Delete(ctx context.Context, userId valobj.Uid, notebookId valobj.Id) error
	// DeleteByNotebookId 幂等：按本清掉所有用户的置顶。
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}
