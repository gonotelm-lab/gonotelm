package notebook

import (
	"context"
	"testing"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type listFakeNotebookRepo struct {
	owned      []*notebookentity.Notebook
	byId       map[valobj.Id]*notebookentity.Notebook
	ownedCalls int
}

func (f *listFakeNotebookRepo) Save(context.Context, *notebookentity.Notebook) error { return nil }

func (f *listFakeNotebookRepo) FindById(_ context.Context, id valobj.Id) (*notebookentity.Notebook, error) {
	return f.byId[id], nil
}

func (f *listFakeNotebookRepo) Exists(context.Context, valobj.Id) (bool, error) { return false, nil }

func (f *listFakeNotebookRepo) ListByOwner(context.Context, valobj.Uid) ([]*notebookentity.Notebook, error) {
	f.ownedCalls++
	return f.owned, nil
}

func (f *listFakeNotebookRepo) FindByIds(_ context.Context, ids []valobj.Id) ([]*notebookentity.Notebook, error) {
	result := make([]*notebookentity.Notebook, 0, len(ids))
	for _, id := range ids {
		if notebook, ok := f.byId[id]; ok {
			result = append(result, notebook)
		}
	}
	return result, nil
}

type listFakeMemberRepo struct {
	members []*notebookentity.NotebookMember
}

func (f *listFakeMemberRepo) Save(context.Context, *notebookentity.NotebookMember) error { return nil }

func (f *listFakeMemberRepo) FindByNotebookIdAndUserId(context.Context, valobj.Id, valobj.Uid) (*notebookentity.NotebookMember, error) {
	return nil, nil
}

func (f *listFakeMemberRepo) ListByNotebookId(context.Context, valobj.Id) ([]*notebookentity.NotebookMember, error) {
	return nil, nil
}

func (f *listFakeMemberRepo) ListByUserId(context.Context, valobj.Uid) ([]*notebookentity.NotebookMember, error) {
	return f.members, nil
}

func (f *listFakeMemberRepo) Delete(context.Context, valobj.Id, valobj.Uid) error { return nil }

func (f *listFakeMemberRepo) DeleteByNotebookId(context.Context, valobj.Id) error { return nil }

func TestListNotebooksAssemblesOwnedAndShared(t *testing.T) {
	userId := valobj.NewUid()
	ownerId := valobj.NewUid()

	owned := []*notebookentity.Notebook{
		newListNotebook(100, ownerId),
		newListNotebook(200, ownerId),
	}
	shared := []*notebookentity.Notebook{
		newListNotebook(300, ownerId),
		newListNotebook(400, ownerId),
	}

	byId := make(map[valobj.Id]*notebookentity.Notebook)
	for _, notebook := range append(owned, shared...) {
		byId[notebook.Id] = notebook
	}

	members := make([]*notebookentity.NotebookMember, 0, len(shared)+1)
	for _, notebook := range shared {
		members = append(members, &notebookentity.NotebookMember{NotebookId: notebook.Id, UserId: userId, Role: vo.RoleViewer})
	}
	members = append(members, &notebookentity.NotebookMember{NotebookId: owned[0].Id, UserId: userId, Role: vo.RoleViewer})

	handler := NewListNotebooksHandler(
		&listFakeNotebookRepo{owned: owned, byId: byId},
		&listFakeMemberRepo{members: members},
	)

	t.Run("all", func(t *testing.T) {
		result, err := handler.Handle(context.Background(), &ListNotebooksHandleQuery{
			UserId: userId,
			Scope:  AccessScopeAll,
			Limit:  10,
		})
		require.NoError(t, err)
		require.Len(t, result.Notebooks, 4)
		assert.Equal(t, int64(100), result.Notebooks[0].CreateTime.Value())
		assert.Equal(t, int64(400), result.Notebooks[3].CreateTime.Value())
		assert.False(t, result.HasMore)
	})

	t.Run("shared", func(t *testing.T) {
		result, err := handler.Handle(context.Background(), &ListNotebooksHandleQuery{
			UserId: userId,
			Scope:  AccessScopeShared,
			Limit:  10,
		})
		require.NoError(t, err)
		require.Len(t, result.Notebooks, 2)
		assert.Equal(t, int64(300), result.Notebooks[0].CreateTime.Value())
	})

	t.Run("owned", func(t *testing.T) {
		result, err := handler.Handle(context.Background(), &ListNotebooksHandleQuery{
			UserId: userId,
			Scope:  AccessScopeOwned,
			Limit:  10,
		})
		require.NoError(t, err)
		require.Len(t, result.Notebooks, 2)
	})

	t.Run("pagination", func(t *testing.T) {
		result, err := handler.Handle(context.Background(), &ListNotebooksHandleQuery{
			UserId: userId,
			Scope:  AccessScopeAll,
			Limit:  2,
			Offset: 1,
		})
		require.NoError(t, err)
		require.Len(t, result.Notebooks, 2)
		assert.Equal(t, int64(200), result.Notebooks[0].CreateTime.Value())
		assert.Equal(t, int64(300), result.Notebooks[1].CreateTime.Value())
		assert.True(t, result.HasMore)
	})
}

func TestListNotebooksDoesNotListPublicOnly(t *testing.T) {
	userId := valobj.NewUid()
	handler := NewListNotebooksHandler(
		&listFakeNotebookRepo{owned: nil, byId: map[valobj.Id]*notebookentity.Notebook{}},
		&listFakeMemberRepo{members: nil},
	)

	result, err := handler.Handle(context.Background(), &ListNotebooksHandleQuery{
		UserId: userId,
		Scope:  AccessScopeAll,
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Notebooks)
}

func newListNotebook(createTimeMs int64, ownerId valobj.Uid) *notebookentity.Notebook {
	base := coreentity.NewBase()
	base.CreateTime = valobj.NewTimeFrom(createTimeMs)
	return &notebookentity.Notebook{
		Base:       base,
		OwnerId:    ownerId,
		Visibility: vo.VisibilityRestricted,
	}
}

var _ notebookrepo.Repository = &listFakeNotebookRepo{}
var _ notebookrepo.MemberRepository = &listFakeMemberRepo{}
