package postgres

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNotebookMemberStoreCRUD(t *testing.T) {
	Convey("NotebookMemberStore CRUD", t, func() {
		store := testNotebookMemberStore
		ctx := t.Context()
		notebookId := database.Id(uuid.NewV7())
		userId := valobj.NewUid()
		now := nowMilli()

		member := &schema.NotebookMember{
			Id:         database.Id(uuid.NewV7()),
			NotebookId: notebookId,
			UserId:     userId,
			Role:       "viewer",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		So(store.Upsert(ctx, member), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("notebook_id = ?", notebookId).Delete(&schema.NotebookMember{}).Error
		})

		got, err := store.GetByNotebookIdAndUserId(ctx, notebookId, userId)
		So(err, ShouldBeNil)
		So(got.Role, ShouldEqual, "viewer")
		So(got.CreatedAt, ShouldEqual, now)

		updated := &schema.NotebookMember{
			Id:         database.Id(uuid.NewV7()),
			NotebookId: notebookId,
			UserId:     userId,
			Role:       "collaborator",
			CreatedAt:  now + 1000,
			UpdatedAt:  now + 1000,
		}
		So(store.Upsert(ctx, updated), ShouldBeNil)

		got, err = store.GetByNotebookIdAndUserId(ctx, notebookId, userId)
		So(err, ShouldBeNil)
		So(got.Role, ShouldEqual, "collaborator")
		So(got.CreatedAt, ShouldEqual, now)
		So(got.UpdatedAt, ShouldEqual, now+1000)

		rows, err := store.ListByNotebookId(ctx, notebookId)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 1)
		So(rows[0].UserId, ShouldEqual, userId)

		rows, err = store.ListByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 1)
		So(rows[0].NotebookId, ShouldEqual, notebookId)

		So(store.Delete(ctx, notebookId, userId), ShouldBeNil)

		_, err = store.GetByNotebookIdAndUserId(ctx, notebookId, userId)
		So(err, ShouldNotBeNil)
	})
}

func TestNotebookMemberStoreDeleteByNotebookId(t *testing.T) {
	Convey("NotebookMemberStore DeleteByNotebookId", t, func() {
		store := testNotebookMemberStore
		ctx := t.Context()
		notebookId := database.Id(uuid.NewV7())
		now := nowMilli()

		for i := 0; i < 2; i++ {
			So(store.Upsert(ctx, &schema.NotebookMember{
				Id:         database.Id(uuid.NewV7()),
				NotebookId: notebookId,
				UserId:     valobj.NewUid(),
				Role:       "viewer",
				CreatedAt:  now,
				UpdatedAt:  now,
			}), ShouldBeNil)
		}
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("notebook_id = ?", notebookId).Delete(&schema.NotebookMember{}).Error
		})

		rows, err := store.ListByNotebookId(ctx, notebookId)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)

		So(store.DeleteByNotebookId(ctx, notebookId), ShouldBeNil)

		rows, err = store.ListByNotebookId(ctx, notebookId)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 0)
	})
}
