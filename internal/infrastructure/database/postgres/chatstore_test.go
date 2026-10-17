package postgres

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChatStoreCRUD(t *testing.T) {
	Convey("ChatStore create / get / list / delete", t, func() {
		store := testChatStore
		ctx := t.Context()
		notebookId := uuid.NewV7()
		ownerId := valobj.NewUid()

		chat := &schema.Chat{
			Id:         uuid.NewV7(),
			NotebookId: notebookId,
			OwnerId:    ownerId,
			UpdatedAt:  nowMilli(),
		}
		So(store.Create(ctx, chat), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", chat.Id).Delete(&schema.Chat{}).Error
		})

		got, err := store.GetById(ctx, chat.Id)
		So(err, ShouldBeNil)
		So(got.NotebookId, ShouldEqual, notebookId)
		So(got.OwnerId, ShouldEqual, ownerId)

		gotByOwner, err := store.GetByNotebookIdAndOwnerId(ctx, notebookId, ownerId)
		So(err, ShouldBeNil)
		So(gotByOwner.Id, ShouldEqual, chat.Id)

		byNotebook, err := store.ListByNotebookId(ctx, notebookId)
		So(err, ShouldBeNil)
		So(byNotebook, ShouldHaveLength, 1)

		byOwner, err := store.ListByOwnerId(ctx, ownerId, 10, 0)
		So(err, ShouldBeNil)
		So(byOwner, ShouldHaveLength, 1)
		So(byOwner[0].Id, ShouldEqual, chat.Id)

		So(store.DeleteById(ctx, chat.Id), ShouldBeNil)

		_, err = store.GetById(ctx, chat.Id)
		So(errors.Is(err, errors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestChatStoreListByOwnerIdRejectsBadPagination(t *testing.T) {
	Convey("ChatStore ListByOwnerId rejects invalid pagination", t, func() {
		store := testChatStore
		ctx := t.Context()

		_, err := store.ListByOwnerId(ctx, valobj.NewUid(), 0, 0)
		So(err, ShouldNotBeNil)

		_, err = store.ListByOwnerId(ctx, valobj.NewUid(), 1, -1)
		So(err, ShouldNotBeNil)
	})
}

func TestChatStoreDeleteByNotebookId(t *testing.T) {
	Convey("ChatStore DeleteByNotebookId only deletes the target notebook", t, func() {
		store := testChatStore
		ctx := t.Context()
		targetNotebook, otherNotebook := uuid.NewV7(), uuid.NewV7()

		chats := []*schema.Chat{
			{Id: uuid.NewV7(), NotebookId: targetNotebook, OwnerId: valobj.NewUid(), UpdatedAt: nowMilli()},
			{Id: uuid.NewV7(), NotebookId: targetNotebook, OwnerId: valobj.NewUid(), UpdatedAt: nowMilli()},
			{Id: uuid.NewV7(), NotebookId: otherNotebook, OwnerId: valobj.NewUid(), UpdatedAt: nowMilli()},
		}
		for _, chat := range chats {
			So(store.Create(ctx, chat), ShouldBeNil)
		}
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).
				Where("notebook_id IN ?", []database.Id{targetNotebook, otherNotebook}).
				Delete(&schema.Chat{}).Error
		})

		So(store.DeleteByNotebookId(ctx, targetNotebook), ShouldBeNil)

		left, err := store.ListByNotebookId(ctx, targetNotebook)
		So(err, ShouldBeNil)
		So(left, ShouldBeEmpty)

		kept, err := store.ListByNotebookId(ctx, otherNotebook)
		So(err, ShouldBeNil)
		So(kept, ShouldHaveLength, 1)
	})
}
