package repository

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNotebookStore struct {
	database.NotebookStore

	ids          []database.Id
	excludeOwner valobj.Uid
	limit        int
	offset       int
	order        database.NotebookListOrder
	rows         []*schema.Notebook
}

func (f *fakeNotebookStore) PageListByIdsExcludingOwner(
	_ context.Context,
	ids []database.Id,
	excludeOwnerId valobj.Uid,
	limit, offset int,
	order database.NotebookListOrder,
) ([]*schema.Notebook, error) {
	f.ids = ids
	f.excludeOwner = excludeOwnerId
	f.limit = limit
	f.offset = offset
	f.order = order

	return f.rows, nil
}

type fakeNotebookMemberStore struct {
	database.NotebookMemberStore
	members []*schema.NotebookMember
}

func (f *fakeNotebookMemberStore) ListByUserId(
	context.Context,
	valobj.Uid,
) ([]*schema.NotebookMember, error) {
	return f.members, nil
}

type fakeSourceStore struct {
	database.SourceStore
}

func (f *fakeSourceStore) BatchCountByNotebookIds(
	context.Context,
	[]database.Id,
) (map[database.Id]int64, error) {
	return map[database.Id]int64{}, nil
}

func TestNotebookRepositoryListShared(t *testing.T) {
	userId := valobj.NewUid()
	notebookId := database.Id(uuid.NewV7())
	otherNotebookId := database.Id(uuid.NewV7())

	notebookStore := &fakeNotebookStore{rows: []*schema.Notebook{{
		Id:        notebookId,
		OwnerId:   valobj.NewUid(),
		UpdatedAt: 1000,
	}}}
	memberStore := &fakeNotebookMemberStore{members: []*schema.NotebookMember{
		{NotebookId: notebookId},
		{NotebookId: notebookId},
		{NotebookId: otherNotebookId},
	}}

	repo := NewNotebookRepository(notebookStore, memberStore, &fakeSourceStore{})

	rows, err := repo.PageListShared(context.Background(), userId, &notebookrepo.PageListSpec{
		Limit:  5,
		Offset: 10,
		Order:  notebookrepo.ListOrderCreateTime,
	}, nil)
	require.NoError(t, err)

	assert.Equal(t, []database.Id{notebookId, otherNotebookId}, notebookStore.ids, "成员关系去重后按 id 查询")
	assert.Equal(t, userId, notebookStore.excludeOwner, "必须排除本人拥有的 notebook")
	assert.Equal(t, 5, notebookStore.limit)
	assert.Equal(t, 10, notebookStore.offset)
	assert.Equal(t, database.NotebookListOrderCreateTime, notebookStore.order)
	require.Len(t, rows, 1)
	assert.Equal(t, notebookId, rows[0].Id)
}

func TestNotebookRepositoryListSharedNoMembership(t *testing.T) {
	notebookStore := &fakeNotebookStore{}
	repo := NewNotebookRepository(notebookStore, &fakeNotebookMemberStore{}, &fakeSourceStore{})

	rows, err := repo.PageListShared(context.Background(), valobj.NewUid(), &notebookrepo.PageListSpec{Limit: 5}, nil)
	require.NoError(t, err)

	assert.Empty(t, rows)
	assert.Nil(t, notebookStore.ids, "没有成员关系时不应查询 notebook store")
}
