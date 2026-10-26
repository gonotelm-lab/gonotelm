package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
)

type Repository interface {
	Save(ctx context.Context, ns *notebookentity.Notebook) error
	FindById(ctx context.Context, id valobj.Id) (*notebookentity.Notebook, error)
	Exists(ctx context.Context, id valobj.Id) (bool, error)
	// ListByIds 按 id 批量查询；ids 为空时返回空切片；不保证顺序。
	ListByIds(ctx context.Context, ids []valobj.Id) ([]*notebookentity.Notebook, error)
	// ListByOwner 分页返回 owner 的 notebook。
	ListByOwner(ctx context.Context, ownerId valobj.Uid, spec *ListSpec) ([]*notebookentity.Notebook, error)
	// ListShared 分页返回显式授权给 userId、且 owner 不是本人的 notebook。
	ListShared(ctx context.Context, userId valobj.Uid, spec *ListSpec) ([]*notebookentity.Notebook, error)
}
