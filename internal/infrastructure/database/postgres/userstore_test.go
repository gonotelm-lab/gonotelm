package postgres

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestUserStoreCRUD(t *testing.T) {
	Convey("UserStore CRUD", t, func() {
		store := testUserStore
		ctx := t.Context()

		user := &schema.User{
			Id:        valobj.NewUid(),
			Email:     uuid.NewV7().String() + "@example.com",
			Nickname:  "nick",
			Status:    "active",
			Avatar:    "https://example.com/a.png",
			Provider:  "github",
			Sub:       uuid.NewV7().String(),
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		}

		err := store.Create(ctx, user)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", user.Id).Delete(&schema.User{}).Error
		})

		gotById, err := store.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(gotById, ShouldNotBeNil)
		So(gotById.Id, ShouldEqual, user.Id)
		So(gotById.Email, ShouldEqual, user.Email)
		So(gotById.Nickname, ShouldEqual, user.Nickname)
		So(gotById.Provider, ShouldEqual, user.Provider)
		So(gotById.Sub, ShouldEqual, user.Sub)

		gotBySub, err := store.GetByProviderAndSub(ctx, user.Provider, user.Sub)
		So(err, ShouldBeNil)
		So(gotBySub, ShouldNotBeNil)
		So(gotBySub.Id, ShouldEqual, user.Id)

		user.Nickname = "nick_updated"
		user.UpdatedAt++
		err = store.Upsert(ctx, user)
		So(err, ShouldBeNil)

		gotAfterUpsert, err := store.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(gotAfterUpsert.Nickname, ShouldEqual, "nick_updated")
	})
}

func TestUserStoreGetNotFound(t *testing.T) {
	Convey("UserStore get not found", t, func() {
		store := testUserStore
		ctx := t.Context()

		_, err := store.GetById(ctx, valobj.NewUid())
		So(err, ShouldNotBeNil)

		_, err = store.GetByProviderAndSub(ctx, "github", uuid.NewV7().String())
		So(err, ShouldNotBeNil)
	})
}
