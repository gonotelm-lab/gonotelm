package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ChatRepository interface {
	Save(ctx context.Context, chat *entity.Chat) error
	FindById(ctx context.Context, id valobj.Id) (*entity.Chat, error)
	FindByNotebookIdAndOwnerId(ctx context.Context, notebookId valobj.Id, ownerId valobj.Uid) (*entity.Chat, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id) ([]*entity.Chat, error)
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}

type PageListSpecOrder int

const (
	// 默认排序: seq_no 从大到小 (最新在前)
	PageListSpecOrderSeqNoDesc PageListSpecOrder = 0
	// seq_no 从小到大 (最早在前)
	PageListSpecOrderSeqNoAsc PageListSpecOrder = 1
)

type PageListSpec struct {
	Offset int
	Limit  int
	Order  PageListSpecOrder
}

func (s *PageListSpec) Validate() error {
	if s.Limit <= 0 || s.Offset < 0 {
		return xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", s.Limit, s.Offset)
	}

	switch s.Order {
	case PageListSpecOrderSeqNoDesc, PageListSpecOrderSeqNoAsc:
		return nil
	default:
		return xerror.ErrParams.Msgf("invalid order: %d", s.Order)
	}
}
