package service

import (
	"context"
	"testing"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNotebookRepo struct {
	notebooks map[valobj.Id]*notebookentity.Notebook
}

func (f *fakeNotebookRepo) Save(context.Context, *notebookentity.Notebook) error { return nil }

func (f *fakeNotebookRepo) FindById(_ context.Context, id valobj.Id) (*notebookentity.Notebook, error) {
	notebook, ok := f.notebooks[id]
	if !ok {
		return nil, notebookerrors.ErrNotebookNotFound
	}
	return notebook, nil
}

func (f *fakeNotebookRepo) Exists(_ context.Context, id valobj.Id) (bool, error) {
	_, ok := f.notebooks[id]
	return ok, nil
}

func (f *fakeNotebookRepo) ListByOwner(context.Context, valobj.Uid, *notebookrepo.ListSpec) ([]*notebookentity.Notebook, error) {
	return nil, nil
}

func (f *fakeNotebookRepo) ListShared(context.Context, valobj.Uid, *notebookrepo.ListSpec) ([]*notebookentity.Notebook, error) {
	return nil, nil
}

type fakeMemberRepo struct {
	members map[valobj.Id]map[valobj.Uid]*notebookentity.NotebookMember
}

func (f *fakeMemberRepo) Save(context.Context, *notebookentity.NotebookMember) error { return nil }

func (f *fakeMemberRepo) FindByNotebookIdAndUserId(
	_ context.Context,
	notebookId valobj.Id,
	userId valobj.Uid,
) (*notebookentity.NotebookMember, error) {
	if byUser, ok := f.members[notebookId]; ok {
		if member, ok := byUser[userId]; ok {
			return member, nil
		}
	}
	return nil, errors.ErrNoRecord
}

func (f *fakeMemberRepo) ListByNotebookId(context.Context, valobj.Id) ([]*notebookentity.NotebookMember, error) {
	return nil, nil
}

func (f *fakeMemberRepo) ListByUserId(context.Context, valobj.Uid) ([]*notebookentity.NotebookMember, error) {
	return nil, nil
}

func (f *fakeMemberRepo) Delete(context.Context, valobj.Id, valobj.Uid) error { return nil }

func (f *fakeMemberRepo) DeleteByNotebookId(context.Context, valobj.Id) error { return nil }

func TestAccessServiceResolve(t *testing.T) {
	ownerId := valobj.NewUid()
	otherId := valobj.NewUid()
	notebookId := valobj.NewId()

	tests := []struct {
		name       string
		visibility vo.Visibility
		memberRole *vo.Role
		userId     valobj.Uid
		expectRole vo.Role
	}{
		{"owner", vo.VisibilityRestricted, nil, ownerId, vo.RoleOwner},
		{"collaborator", vo.VisibilityRestricted, rolePtr(vo.RoleCollaborator), otherId, vo.RoleCollaborator},
		{"viewer", vo.VisibilityRestricted, rolePtr(vo.RoleViewer), otherId, vo.RoleViewer},
		{"public viewer", vo.VisibilityPublic, nil, otherId, vo.RoleViewer},
		{"restricted none", vo.VisibilityRestricted, nil, otherId, vo.RoleNone},
		{"empty visibility none", "", nil, otherId, vo.RoleNone},
		{"empty role falls through", vo.VisibilityRestricted, rolePtr(""), otherId, vo.RoleNone},
		{"invalid role falls through", vo.VisibilityPublic, rolePtr("invalid"), otherId, vo.RoleViewer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notebook := &notebookentity.Notebook{
				Base:       newBase(notebookId),
				OwnerId:    ownerId,
				Visibility: tt.visibility,
			}
			memberRepo := &fakeMemberRepo{members: map[valobj.Id]map[valobj.Uid]*notebookentity.NotebookMember{}}
			if tt.memberRole != nil {
				memberRepo.members[notebookId] = map[valobj.Uid]*notebookentity.NotebookMember{
					otherId: {
						Base:       newBase(notebookId),
						NotebookId: notebookId,
						UserId:     otherId,
						Role:       *tt.memberRole,
					},
				}
			}

			service := NewAccessService(
				&fakeNotebookRepo{notebooks: map[valobj.Id]*notebookentity.Notebook{notebookId: notebook}},
				memberRepo,
			)

			access, err := service.Resolve(context.Background(), notebookId, tt.userId)
			require.NoError(t, err)
			assert.Equal(t, tt.expectRole, access.Role)
		})
	}
}

func rolePtr(role vo.Role) *vo.Role { return &role }

func newBase(id valobj.Id) coreentity.Base {
	base := coreentity.NewBase()
	base.Id = id
	return base
}
