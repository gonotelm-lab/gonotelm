package redis

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/ulid"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func newTestUser() (*schema.User, string) {
	id := ulid.New().String()

	return &schema.User{
		Id:        id,
		Email:     "nick@example.com",
		Nickname:  "nick",
		Status:    "active",
		Avatar:    "stkv1-avatar",
		Provider:  "github",
		Sub:       uuid.NewV7().String(),
		CreatedAt: 1700000000000,
		UpdatedAt: 1700000000001,
	}, id
}

func TestUserCacheImplSetGetById(t *testing.T) {
	Convey("UserCache Set then GetById returns the stored user", t, func() {
		user, id := newTestUser()

		So(testUserCache.Set(t.Context(), user), ShouldBeNil)

		got, err := testUserCache.GetById(t.Context(), id)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.Id, ShouldEqual, id)
		So(got.Email, ShouldEqual, "nick@example.com")
		So(got.Nickname, ShouldEqual, "nick")
		So(got.Status, ShouldEqual, "active")
		So(got.Avatar, ShouldEqual, "stkv1-avatar")
		So(got.Provider, ShouldEqual, "github")
		So(got.Sub, ShouldEqual, user.Sub)
		So(got.CreatedAt, ShouldEqual, 1700000000000)
		So(got.UpdatedAt, ShouldEqual, 1700000000001)
	})
}

func TestUserCacheImplGetMissing(t *testing.T) {
	Convey("UserCache GetById on a missing userId returns nil without error", t, func() {
		got, err := testUserCache.GetById(t.Context(), ulid.New().String())
		So(err, ShouldBeNil)
		So(got, ShouldBeNil)
	})
}

func TestUserCacheImplSetOverwrites(t *testing.T) {
	Convey("UserCache Set overwrites the previously cached user", t, func() {
		user, id := newTestUser()
		So(testUserCache.Set(t.Context(), user), ShouldBeNil)

		user.Nickname = "renamed"
		user.Status = "banned"
		So(testUserCache.Set(t.Context(), user), ShouldBeNil)

		got, err := testUserCache.GetById(t.Context(), id)
		So(err, ShouldBeNil)
		So(got.Nickname, ShouldEqual, "renamed")
		So(got.Status, ShouldEqual, "banned")
	})
}

func TestUserCacheImplDelete(t *testing.T) {
	Convey("UserCache Delete removes the cached user", t, func() {
		user, id := newTestUser()
		So(testUserCache.Set(t.Context(), user), ShouldBeNil)

		So(testUserCache.Delete(t.Context(), id), ShouldBeNil)

		got, err := testUserCache.GetById(t.Context(), id)
		So(err, ShouldBeNil)
		So(got, ShouldBeNil)
	})
}

func TestUserCacheImplDeleteMissing(t *testing.T) {
	Convey("UserCache Delete on a missing userId is a no-op", t, func() {
		So(testUserCache.Delete(t.Context(), ulid.New().String()), ShouldBeNil)
	})
}
