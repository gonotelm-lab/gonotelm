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
	// PageListByOwner 分页返回 owner 的 notebook；excludeIds 非空时排除这些 id。
	PageListByOwner(ctx context.Context, ownerId valobj.Uid, spec *PageListSpec, excludeIds []valobj.Id) ([]*notebookentity.Notebook, error)
	// PageListShared 分页返回显式授权给 userId、且 owner 不是本人的 notebook；excludeIds 非空时排除这些 id。
	PageListShared(ctx context.Context, userId valobj.Uid, spec *PageListSpec, excludeIds []valobj.Id) ([]*notebookentity.Notebook, error)
}
