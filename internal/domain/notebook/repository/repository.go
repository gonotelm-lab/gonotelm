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
	// ListByOwner 返回 owner 的全部 notebook，不分页；列表由 application 层组装。
	ListByOwner(ctx context.Context, ownerId valobj.Uid) ([]*notebookentity.Notebook, error)
	// FindByIds 按 id 批量查询；ids 为空时返回空切片。
	FindByIds(ctx context.Context, ids []valobj.Id) ([]*notebookentity.Notebook, error)
}
