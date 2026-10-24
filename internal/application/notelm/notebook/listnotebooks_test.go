package notebook

import (
	"context"
	"testing"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	idententity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNotebookRepo struct {
	ownedRows  []*notebookentity.Notebook
	sharedRows []*notebookentity.Notebook

	ownedSpec  *notebookrepo.ListSpec
	sharedSpec *notebookrepo.ListSpec
	sharedCall bool
}

func (f *fakeNotebookRepo) Save(context.Context, *notebookentity.Notebook) error { return nil }

func (f *fakeNotebookRepo) FindById(context.Context, valobj.Id) (*notebookentity.Notebook, error) {
	return nil, nil
}

func (f *fakeNotebookRepo) Exists(context.Context, valobj.Id) (bool, error) { return false, nil }

func (f *fakeNotebookRepo) ListByOwner(
	_ context.Context,
	_ valobj.Uid,
	spec *notebookrepo.ListSpec,
) ([]*notebookentity.Notebook, error) {
	f.ownedSpec = spec

	return f.ownedRows, nil
}

func (f *fakeNotebookRepo) ListShared(
	_ context.Context,
	_ valobj.Uid,
	spec *notebookrepo.ListSpec,
) ([]*notebookentity.Notebook, error) {
	f.sharedCall = true
	f.sharedSpec = spec

	return f.sharedRows, nil
}

type fakeUserRepo struct {
	identrepo.UserRepository
	users []*idententity.User
}

func (f *fakeUserRepo) BatchGetById(context.Context, []valobj.Uid) ([]*idententity.User, error) {
	return f.users, nil
}

func newNotebook(ownerId valobj.Uid) *notebookentity.Notebook {
	return &notebookentity.Notebook{
		Base:    coreentity.NewBase(),
		Name:    "notebook",
		OwnerId: ownerId,
	}
}

func TestListNotebooksHandleOwned(t *testing.T) {
	ownerId := valobj.NewUid()
	nb1 := newNotebook(ownerId)
	nb2 := newNotebook(ownerId)

	repo := &fakeNotebookRepo{ownedRows: []*notebookentity.Notebook{nb1, nb2}}
	handler := NewListNotebooksHandler(repo, &fakeUserRepo{
		users: []*idententity.User{{Id: ownerId, Nickname: "owner"}},
	}, nil)

	ctx := pkgcontext.WithUserId(context.Background(), ownerId)
	result, err := handler.Handle(ctx, &ListNotebooksHandleQuery{
		Scope:  AccessScopeOwned,
		Limit:  2,
		Offset: 4,
		SortBy: SortByLastActive,
	})
	require.NoError(t, err)

	assert.False(t, repo.sharedCall)
	require.NotNil(t, repo.ownedSpec)
	assert.Equal(t, 3, repo.ownedSpec.Limit, "多取一条用于判断 has_more")
	assert.Equal(t, 4, repo.ownedSpec.Offset)
	assert.Equal(t, notebookrepo.ListOrderLastActive, repo.ownedSpec.Order)

	assert.False(t, result.HasMore)
	assert.Len(t, result.Notebooks, 2)
	assert.Equal(t, 2, result.Limit)
	assert.Equal(t, 4, result.Offset)
	require.NotNil(t, result.Notebooks[0].Owner)
	assert.Equal(t, "owner", result.Notebooks[0].Owner.Nickname)
}

func TestListNotebooksHandleOwnedHasMore(t *testing.T) {
	ownerId := valobj.NewUid()
	repo := &fakeNotebookRepo{ownedRows: []*notebookentity.Notebook{
		newNotebook(ownerId),
		newNotebook(ownerId),
		newNotebook(ownerId),
	}}
	handler := NewListNotebooksHandler(repo, &fakeUserRepo{
		users: []*idententity.User{{Id: ownerId, Nickname: "owner"}},
	}, nil)

	ctx := pkgcontext.WithUserId(context.Background(), ownerId)
	result, err := handler.Handle(ctx, &ListNotebooksHandleQuery{
		Scope:  AccessScopeOwned,
		Limit:  2,
		Offset: 0,
		SortBy: SortByCreateTime,
	})
	require.NoError(t, err)

	assert.True(t, result.HasMore)
	assert.Len(t, result.Notebooks, 2)
	assert.Equal(t, notebookrepo.ListOrderCreateTime, repo.ownedSpec.Order)
}

func TestListNotebooksHandleShared(t *testing.T) {
	userId := valobj.NewUid()
	ownerId := valobj.NewUid()

	repo := &fakeNotebookRepo{sharedRows: []*notebookentity.Notebook{newNotebook(ownerId)}}
	handler := NewListNotebooksHandler(repo, &fakeUserRepo{
		users: []*idententity.User{{Id: ownerId, Nickname: "owner"}},
	}, nil)

	ctx := pkgcontext.WithUserId(context.Background(), userId)
	result, err := handler.Handle(ctx, &ListNotebooksHandleQuery{
		Scope:  AccessScopeShared,
		Limit:  10,
		Offset: 0,
		SortBy: SortByCreateTime,
	})
	require.NoError(t, err)

	assert.True(t, repo.sharedCall)
	assert.Nil(t, repo.ownedSpec, "shared 不应查 owned")
	assert.Len(t, result.Notebooks, 1)
	assert.Equal(t, ownerId, result.Notebooks[0].OwnerId)
	require.NotNil(t, result.Notebooks[0].Owner)
	assert.Equal(t, "owner", result.Notebooks[0].Owner.Nickname)
}

func TestListNotebooksHandleInvalidScope(t *testing.T) {
	handler := NewListNotebooksHandler(&fakeNotebookRepo{}, &fakeUserRepo{}, nil)

	ctx := pkgcontext.WithUserId(context.Background(), valobj.NewUid())
	_, err := handler.Handle(ctx, &ListNotebooksHandleQuery{
		Scope: AccessScope(99),
		Limit: 10,
	})
	require.Error(t, err)
}
