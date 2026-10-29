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
	ListByIds(ctx context.Context, ids []valobj.Id) ([]*notebookentity.Notebook, error)
	// ListByOwner 分页返回 owner 的 notebook；excludeIds 非空时排除这些 id。
	ListByOwner(ctx context.Context, ownerId valobj.Uid, spec *ListSpec, excludeIds []valobj.Id) ([]*notebookentity.Notebook, error)
	// ListShared 分页返回显式授权给 userId、且 owner 不是本人的 notebook；excludeIds 非空时排除这些 id。
	ListShared(ctx context.Context, userId valobj.Uid, spec *ListSpec, excludeIds []valobj.Id) ([]*notebookentity.Notebook, error)
}
