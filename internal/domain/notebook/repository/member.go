package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
)

type MemberRepository interface {
	// Save 是 upsert：冲突时更新 role / updated_at，不覆盖 created_at。
	Save(ctx context.Context, member *notebookentity.NotebookMember) error
	// FindByNotebookIdAndUserId 不存在时返回 ErrNoRecord。
	FindByNotebookIdAndUserId(
		ctx context.Context,
		notebookId valobj.Id,
		userId valobj.Uid,
	) (*notebookentity.NotebookMember, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id) ([]*notebookentity.NotebookMember, error)
	ListByUserId(ctx context.Context, userId valobj.Uid) ([]*notebookentity.NotebookMember, error)
	Delete(ctx context.Context, notebookId valobj.Id, userId valobj.Uid) error
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}
