package postgres

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestUserNotebookPinStoreCRUD(t *testing.T) {
	Convey("UserNotebookPinStore upsert find list count delete", t, func() {
		store := testUserNotebookPinStore
		ctx := t.Context()
		userId := valobj.NewUid()
		nb1 := database.Id(uuid.NewV7())
		nb2 := database.Id(uuid.NewV7())
		t.Cleanup(func() {
			_ = store.Delete(ctx, userId, nb1)
			_ = store.Delete(ctx, userId, nb2)
		})

		So(store.Upsert(ctx, &schema.UserNotebookPin{
			UserId: userId, NotebookId: nb1, PinnedAt: 1000,
		}), ShouldBeNil)
		So(store.Upsert(ctx, &schema.UserNotebookPin{
			UserId: userId, NotebookId: nb2, PinnedAt: 2000,
		}), ShouldBeNil)

		got, err := store.Get(ctx, userId, nb1)
		So(err, ShouldBeNil)
		So(got.PinnedAt, ShouldEqual, 1000)

		So(store.Upsert(ctx, &schema.UserNotebookPin{
			UserId: userId, NotebookId: nb1, PinnedAt: 3000,
		}), ShouldBeNil)
		got, err = store.Get(ctx, userId, nb1)
		So(err, ShouldBeNil)
		So(got.PinnedAt, ShouldEqual, 3000)

		rows, err := store.ListByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].NotebookId, ShouldEqual, nb1)
		So(rows[0].PinnedAt, ShouldEqual, 3000)
		So(rows[1].NotebookId, ShouldEqual, nb2)

		count, err := store.CountByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(count, ShouldEqual, 2)

		count, err = store.CountByUserId(ctx, valobj.NewUid())
		So(err, ShouldBeNil)
		So(count, ShouldEqual, 0)

		So(store.Delete(ctx, userId, nb1), ShouldBeNil)
		So(store.Delete(ctx, userId, nb1), ShouldBeNil) // idempotent

		_, err = store.Get(ctx, userId, nb1)
		So(err, ShouldNotBeNil)

		rows, err = store.ListByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 1)
	})
}

func TestUserNotebookPinStoreDeleteByNotebookId(t *testing.T) {
	Convey("UserNotebookPinStore DeleteByNotebookId clears all users", t, func() {
		store := testUserNotebookPinStore
		ctx := t.Context()
		notebookId := database.Id(uuid.NewV7())
		u1 := valobj.NewUid()
		u2 := valobj.NewUid()
		t.Cleanup(func() {
			_ = store.DeleteByNotebookId(ctx, notebookId)
		})

		So(store.Upsert(ctx, &schema.UserNotebookPin{
			UserId: u1, NotebookId: notebookId, PinnedAt: 1,
		}), ShouldBeNil)
		So(store.Upsert(ctx, &schema.UserNotebookPin{
			UserId: u2, NotebookId: notebookId, PinnedAt: 2,
		}), ShouldBeNil)

		So(store.DeleteByNotebookId(ctx, notebookId), ShouldBeNil)
		So(store.DeleteByNotebookId(ctx, notebookId), ShouldBeNil)

		_, err := store.Get(ctx, u1, notebookId)
		So(err, ShouldNotBeNil)
		_, err = store.Get(ctx, u2, notebookId)
		So(err, ShouldNotBeNil)
	})
}
