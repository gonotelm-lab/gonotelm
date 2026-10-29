package service

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// 笔记本的权限访问控制
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
		return nil, errors.WithMessagef(err, "notebook repo find by id failed, notebook_id=%s", notebookId)
	}

	if notebook.IsOwnedBy(userId) {
		return &notebookentity.Access{Notebook: notebook, Role: vo.RoleOwner}, nil
	}

	if access, err := s.resolveExplicitMember(ctx, notebook, userId); err != nil {
		return nil, err
	} else if access != nil {
		return access, nil
	}

	if notebook.IsPublic() {
		return &notebookentity.Access{Notebook: notebook, Role: vo.RoleViewer}, nil
	}

	return &notebookentity.Access{Notebook: notebook, Role: vo.RoleNone}, nil
}

// ResolveOwnerOrMember 只认 owner / 显式成员（不含 public viewer）；否则返回 ErrNotebookNotFound。
func (s *AccessService) ResolveOwnerOrMember(
	ctx context.Context,
	notebookId valobj.Id,
	userId valobj.Uid,
) (*notebookentity.Access, error) {
	notebook, err := s.notebookRepo.FindById(ctx, notebookId)
	if err != nil {
		if errors.Is(err, notebookerrors.ErrNotebookNotFound) {
			return nil, notebookerrors.ErrNotebookNotFound
		}
		return nil, errors.WithMessagef(err, "notebook repo find by id failed, notebook_id=%s", notebookId)
	}

	if notebook.IsOwnedBy(userId) {
		return &notebookentity.Access{Notebook: notebook, Role: vo.RoleOwner}, nil
	}

	access, err := s.resolveExplicitMember(ctx, notebook, userId)
	if err != nil {
		return nil, err
	}
	if access != nil {
		return access, nil
	}

	return nil, notebookerrors.ErrNotebookNotFound
}

func (s *AccessService) resolveExplicitMember(
	ctx context.Context,
	notebook *notebookentity.Notebook,
	userId valobj.Uid,
) (*notebookentity.Access, error) {
	member, err := s.memberRepo.FindByNotebookIdAndUserId(ctx, notebook.Id, userId)
	if err == nil {
		if member.Role.IsMemberRole() {
			return &notebookentity.Access{Notebook: notebook, Role: member.Role}, nil
		}

		slog.WarnContext(ctx, "ignore notebook member with invalid role",
			slog.String("notebook_id", notebook.Id.String()),
			slog.String("user_id", userId.String()),
			slog.String("role", string(member.Role)),
		)
		return nil, nil
	}
	if !errors.Is(err, errors.ErrNoRecord) {
		return nil, errors.WithMessagef(err, "member repo find failed, notebook_id=%s, user_id=%s", notebook.Id, userId)
	}
	return nil, nil
}
