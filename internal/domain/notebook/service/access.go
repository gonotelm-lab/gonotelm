package service

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type AccessService struct {
	notebookRepo notebookrepo.Repository
	memberRepo   notebookrepo.MemberRepository
}

func NewAccessService(
	notebookRepo notebookrepo.Repository,
	memberRepo notebookrepo.MemberRepository,
) *AccessService {
	return &AccessService{
		notebookRepo: notebookRepo,
		memberRepo:   memberRepo,
	}
}

// Resolve 解析用户对 notebook 的有效角色：
// owner -> 显式成员 -> public viewer -> none。
func (s *AccessService) Resolve(
	ctx context.Context,
	notebookId valobj.Id,
	userId valobj.Uid,
) (*notebookentity.Access, error) {
	notebook, err := s.notebookRepo.FindById(ctx, notebookId)
	if err != nil {
		return nil, err
	}

	if notebook.OwnerId == userId {
		return &notebookentity.Access{Notebook: notebook, Role: vo.RoleOwner}, nil
	}

	member, err := s.memberRepo.FindByNotebookIdAndUserId(ctx, notebookId, userId)
	if err == nil {
		role := vo.NormalizeRole(member.Role)
		if role.IsMemberRole() {
			return &notebookentity.Access{Notebook: notebook, Role: role}, nil
		}

		slog.WarnContext(ctx, "ignore notebook member with invalid role",
			slog.String("notebook_id", notebookId.String()),
			slog.String("user_id", userId.String()),
			slog.String("role", string(member.Role)),
		)
	} else if !errors.Is(err, errors.ErrNoRecord) {
		return nil, err
	}

	if vo.NormalizeVisibility(notebook.Visibility).IsPublic() {
		return &notebookentity.Access{Notebook: notebook, Role: vo.RoleViewer}, nil
	}

	return &notebookentity.Access{Notebook: notebook, Role: vo.RoleNone}, nil
}
