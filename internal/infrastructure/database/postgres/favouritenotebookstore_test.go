package postgres

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestFavouriteNotebookStoreCRUD(t *testing.T) {
	Convey("FavouriteNotebookStore insert ignore list delete cascade", t, func() {
		favStore := testFavouriteStore
		store := testFavouriteNotebookStore
		ctx := t.Context()
		userId := valobj.NewUid()
		favId := database.Id(uuid.NewV7())
		nb1 := database.Id(uuid.NewV7())
		nb2 := database.Id(uuid.NewV7())
		t.Cleanup(func() {
			_ = store.DeleteByFavouriteId(ctx, favId)
			_ = favStore.DeleteById(ctx, favId)
		})

		So(favStore.Upsert(ctx, &schema.Favourite{
			Id: favId, UserId: userId, Name: "f", CreatedAt: 1, UpdatedAt: 1,
		}), ShouldBeNil)

		So(store.InsertIgnore(ctx, &schema.FavouriteNotebook{
			FavouriteId: favId, NotebookId: nb1, FavouritedAt: 1000,
		}), ShouldBeNil)
		So(store.InsertIgnore(ctx, &schema.FavouriteNotebook{
			FavouriteId: favId, NotebookId: nb2, FavouritedAt: 2000,
		}), ShouldBeNil)
		// 幂等：不刷新 favourited_at
		So(store.InsertIgnore(ctx, &schema.FavouriteNotebook{
			FavouriteId: favId, NotebookId: nb1, FavouritedAt: 9999,
		}), ShouldBeNil)

		got, err := store.Get(ctx, favId, nb1)
		So(err, ShouldBeNil)
		So(got.FavouritedAt, ShouldEqual, 1000)

		rows, err := store.ListByFavouriteId(ctx, favId, 10, 0)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].NotebookId, ShouldEqual, nb2)

		count, err := store.CountByFavouriteId(ctx, favId)
		So(err, ShouldBeNil)
		So(count, ShouldEqual, 2)

		So(store.Delete(ctx, favId, []database.Id{nb1, nb2}), ShouldBeNil)
		So(store.Delete(ctx, favId, []database.Id{nb1}), ShouldBeNil)
		count, err = store.CountByFavouriteId(ctx, favId)
		So(err, ShouldBeNil)
		So(count, ShouldEqual, 0)

		So(store.InsertIgnore(ctx, &schema.FavouriteNotebook{
			FavouriteId: favId, NotebookId: nb1, FavouritedAt: 1000,
		}), ShouldBeNil)
		So(store.DeleteByUserId(ctx, userId), ShouldBeNil)
		count, err = store.CountByFavouriteId(ctx, favId)
		So(err, ShouldBeNil)
		So(count, ShouldEqual, 0)
	})
}

func TestFavouriteNotebookStoreListByNotebookIdAndFavouriteIds(t *testing.T) {
	Convey("ListByNotebookIdAndFavouriteIds filters and orders by favourited_at", t, func() {
		favStore := testFavouriteStore
		store := testFavouriteNotebookStore
		ctx := t.Context()
		userId := valobj.NewUid()
		fav1 := database.Id(uuid.NewV7())
		fav2 := database.Id(uuid.NewV7())
		favOther := database.Id(uuid.NewV7())
		nb := database.Id(uuid.NewV7())
		t.Cleanup(func() {
			_ = store.DeleteByFavouriteId(ctx, fav1)
			_ = store.DeleteByFavouriteId(ctx, fav2)
			_ = store.DeleteByFavouriteId(ctx, favOther)
			_ = favStore.DeleteById(ctx, fav1)
			_ = favStore.DeleteById(ctx, fav2)
			_ = favStore.DeleteById(ctx, favOther)
		})

		So(favStore.Upsert(ctx, &schema.Favourite{
			Id: fav1, UserId: userId, Name: "a", CreatedAt: 1, UpdatedAt: 1,
		}), ShouldBeNil)
		So(favStore.Upsert(ctx, &schema.Favourite{
			Id: fav2, UserId: userId, Name: "b", CreatedAt: 2, UpdatedAt: 2,
		}), ShouldBeNil)
		So(favStore.Upsert(ctx, &schema.Favourite{
			Id: favOther, UserId: userId, Name: "c", CreatedAt: 3, UpdatedAt: 3,
		}), ShouldBeNil)

		So(store.InsertIgnore(ctx, &schema.FavouriteNotebook{
			FavouriteId: fav1, NotebookId: nb, FavouritedAt: 1000,
		}), ShouldBeNil)
		So(store.InsertIgnore(ctx, &schema.FavouriteNotebook{
			FavouriteId: fav2, NotebookId: nb, FavouritedAt: 3000,
		}), ShouldBeNil)
		So(store.InsertIgnore(ctx, &schema.FavouriteNotebook{
			FavouriteId: favOther, NotebookId: nb, FavouritedAt: 2000,
		}), ShouldBeNil)

		rows, err := store.ListByNotebookIdAndFavouriteIds(ctx, nb, []database.Id{fav1, fav2})
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].FavouriteId, ShouldEqual, fav2)
		So(rows[1].FavouriteId, ShouldEqual, fav1)

		empty, err := store.ListByNotebookIdAndFavouriteIds(ctx, nb, nil)
		So(err, ShouldBeNil)
		So(len(empty), ShouldEqual, 0)
	})
}
