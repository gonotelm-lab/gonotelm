package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
)

type Repository interface {
	// Create 只做插入，主键冲突即报错，不覆盖已有行。
	Create(ctx context.Context, s *entity.Source) error
	// UpdateExisting 只更新已存在的记录
	UpdateExisting(ctx context.Context, s *entity.Source) (int64, error)
	DeleteById(ctx context.Context, id valobj.Id) error
	FindById(ctx context.Context, id valobj.Id) (*entity.Source, error)
	PageListByNotebookId(ctx context.Context, notebookId valobj.Id, spec *PageListSpec) ([]*entity.Source, error)
	GetByNotebookIdAndIds(ctx context.Context, notebookId valobj.Id, ids []valobj.Id) ([]*entity.Source, error)
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
	BatchDeleteByIds(ctx context.Context, ids []valobj.Id) error
}
