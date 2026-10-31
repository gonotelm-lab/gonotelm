package postgres

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestFavouriteStoreCRUD(t *testing.T) {
	Convey("FavouriteStore upsert list count delete", t, func() {
		store := testFavouriteStore
		ctx := t.Context()
		userId := valobj.NewUid()
		id1 := database.Id(uuid.NewV7())
		id2 := database.Id(uuid.NewV7())
		t.Cleanup(func() {
			_ = store.DeleteByUserId(ctx, userId)
		})

		So(store.Upsert(ctx, &schema.Favourite{
			Id: id1, UserId: userId, Name: "a", CreatedAt: 1000, UpdatedAt: 1000,
		}), ShouldBeNil)
		So(store.Upsert(ctx, &schema.Favourite{
			Id: id2, UserId: userId, Name: "b", CreatedAt: 2000, UpdatedAt: 2000,
		}), ShouldBeNil)

		got, err := store.GetById(ctx, id1)
		So(err, ShouldBeNil)
		So(got.Name, ShouldEqual, "a")

		So(store.Upsert(ctx, &schema.Favourite{
			Id: id1, UserId: userId, Name: "a2", CreatedAt: 1000, UpdatedAt: 3000,
		}), ShouldBeNil)
		got, err = store.GetById(ctx, id1)
		So(err, ShouldBeNil)
		So(got.Name, ShouldEqual, "a2")
		So(got.UpdatedAt, ShouldEqual, 3000)
		So(got.CreatedAt, ShouldEqual, 1000)

		rows, err := store.ListByUserId(ctx, userId, 10, 0)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].Id, ShouldEqual, id2)
		So(rows[1].Id, ShouldEqual, id1)

		count, err := store.CountByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(count, ShouldEqual, 2)

		So(store.DeleteById(ctx, id1), ShouldBeNil)
		So(store.DeleteById(ctx, id1), ShouldBeNil)
		count, err = store.CountByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(count, ShouldEqual, 1)
	})
}
