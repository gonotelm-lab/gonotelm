package pin

import (
	"context"
	"testing"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNotebookRepo struct {
	notebookrepo.Repository
	nb *notebookentity.Notebook
}

func (f *fakeNotebookRepo) FindById(context.Context, valobj.Id) (*notebookentity.Notebook, error) {
	if f.nb == nil {
		return nil, notebookerrors.ErrNotebookNotFound
	}
	return f.nb, nil
}

type fakeMemberRepo struct {
	notebookrepo.MemberRepository
	member *notebookentity.NotebookMember
	err    error
}

func (f *fakeMemberRepo) FindByNotebookIdAndUserId(
	context.Context, valobj.Id, valobj.Uid,
) (*notebookentity.NotebookMember, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.member, nil
}

type fakePinRepo struct{}

func (fakePinRepo) Save(context.Context, *pinentity.UserNotebookPin) error { return nil }
func (fakePinRepo) Find(context.Context, valobj.Uid, valobj.Id) (*pinentity.UserNotebookPin, error) {
	return nil, errors.ErrNoRecord
}
func (fakePinRepo) ListByUserId(context.Context, valobj.Uid) ([]*pinentity.UserNotebookPin, error) {
	return nil, nil
}
func (fakePinRepo) CountByUserId(context.Context, valobj.Uid) (int64, error) {
	return 0, nil
}
func (fakePinRepo) Delete(context.Context, valobj.Uid, valobj.Id) error { return nil }
func (fakePinRepo) DeleteByNotebookId(context.Context, valobj.Id) error { return nil }

func TestResolveAccessOwner(t *testing.T) {
	owner := valobj.NewUid()
	nb := &notebookentity.Notebook{
		Base:       coreentity.NewBase(),
		OwnerId:    owner,
		Visibility: vo.VisibilityPublic,
	}
	r := &scopeResolver{
		notebookRepo: &fakeNotebookRepo{nb: nb},
		memberRepo:   &fakeMemberRepo{err: errors.ErrNoRecord},
		pinRepo:      fakePinRepo{},
	}
	err := r.resolveAccess(context.Background(), owner, nb.Id)
	require.NoError(t, err)
}

func TestResolveAccessExplicitMember(t *testing.T) {
	owner := valobj.NewUid()
	member := valobj.NewUid()
	nb := &notebookentity.Notebook{
		Base:       coreentity.NewBase(),
		OwnerId:    owner,
		Visibility: vo.VisibilityRestricted,
	}
	r := &scopeResolver{
		notebookRepo: &fakeNotebookRepo{nb: nb},
		memberRepo: &fakeMemberRepo{member: &notebookentity.NotebookMember{
			UserId: member, Role: vo.RoleViewer,
		}},
		pinRepo: fakePinRepo{},
	}
	err := r.resolveAccess(context.Background(), member, nb.Id)
	require.NoError(t, err)
}

func TestResolveAccessPublicOnlyDenied(t *testing.T) {
	owner := valobj.NewUid()
	stranger := valobj.NewUid()
	nb := &notebookentity.Notebook{
		Base:       coreentity.NewBase(),
		OwnerId:    owner,
		Visibility: vo.VisibilityPublic,
	}
	r := &scopeResolver{
		notebookRepo: &fakeNotebookRepo{nb: nb},
		memberRepo:   &fakeMemberRepo{err: errors.ErrNoRecord},
		pinRepo:      fakePinRepo{},
	}
	err := r.resolveAccess(context.Background(), stranger, nb.Id)
	require.Error(t, err)
	assert.True(t, errors.Is(err, notebookerrors.ErrNotebookNotFound))
}
