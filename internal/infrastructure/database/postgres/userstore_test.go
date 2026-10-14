package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func newTestUserRow(passwordHash string) *schema.User {
	now := time.Now().UnixMilli()

	return &schema.User{
		Id:              valobj.NewUid(),
		Email:           uuid.NewV7().String() + "@example.com",
		EmailVerifiedAt: now,
		Nickname:        "nick",
		Status:          "active",
		Avatar:          "stkv1-avatar",
		PasswordHash:    passwordHash,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func cleanupTestUser(t *testing.T, id valobj.Uid) {
	t.Helper()
	t.Cleanup(func() {
		_ = testDB.Where("id = ?", id).Delete(&schema.User{}).Error
	})
}

func TestUserStoreCreateAndRead(t *testing.T) {
	Convey("UserStore Create then read back by id and email", t, func() {
		ctx := t.Context()
		user := newTestUserRow("")
		cleanupTestUser(t, user.Id)

		So(testUserStore.Create(ctx, user), ShouldBeNil)

		byId, err := testUserStore.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(byId.Email, ShouldEqual, user.Email)
		So(byId.EmailVerifiedAt, ShouldEqual, user.EmailVerifiedAt)
		So(byId.Nickname, ShouldEqual, user.Nickname)
		So(byId.PasswordHash, ShouldEqual, "")

		byEmail, err := testUserStore.GetByEmail(ctx, user.Email)
		So(err, ShouldBeNil)
		So(byEmail.Id, ShouldEqual, user.Id)

		hash, err := testUserStore.GetPasswordHash(ctx, user.Id)
		So(err, ShouldBeNil)
		So(hash, ShouldEqual, "")
	})
}

func TestUserStoreDuplicateEmailFails(t *testing.T) {
	Convey("UserStore rejects a second account with the same email", t, func() {
		ctx := t.Context()
		first := newTestUserRow("")
		cleanupTestUser(t, first.Id)
		So(testUserStore.Create(ctx, first), ShouldBeNil)

		second := newTestUserRow("")
		second.Email = first.Email
		So(testUserStore.Create(ctx, second), ShouldNotBeNil)
	})
}

func TestUserStoreReadMissing(t *testing.T) {
	Convey("UserStore returns ErrNoRecord for unknown id and email", t, func() {
		ctx := t.Context()

		_, err := testUserStore.GetById(ctx, valobj.NewUid())
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		_, err = testUserStore.GetByEmail(ctx, "nobody@example.com")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		_, err = testUserStore.GetPasswordHash(ctx, valobj.NewUid())
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestUserStoreNarrowUpdates(t *testing.T) {
	Convey("UserStore updates only the intended columns", t, func() {
		ctx := t.Context()
		user := newTestUserRow("")
		cleanupTestUser(t, user.Id)
		So(testUserStore.Create(ctx, user), ShouldBeNil)

		now := time.Now().UnixMilli() + 1

		So(testUserStore.UpdateProfile(ctx, &schema.UserUpdateProfileParams{
			Id: user.Id, Nickname: "renamed", UpdatedAt: now,
		}), ShouldBeNil)

		got, err := testUserStore.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(got.Nickname, ShouldEqual, "renamed")
		So(got.Email, ShouldEqual, user.Email)
		So(got.Status, ShouldEqual, "active")

		So(testUserStore.UpdateAvatar(ctx, &schema.UserUpdateAvatarParams{
			Id: user.Id, Avatar: "stkv1-new-avatar", UpdatedAt: now,
		}), ShouldBeNil)
		got, err = testUserStore.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(got.Avatar, ShouldEqual, "stkv1-new-avatar")

		So(testUserStore.UpdatePasswordHash(ctx, &schema.UserUpdatePasswordParams{
			Id: user.Id, PasswordHash: "bcrypt-hash", UpdatedAt: now,
		}), ShouldBeNil)
		hash, err := testUserStore.GetPasswordHash(ctx, user.Id)
		So(err, ShouldBeNil)
		So(hash, ShouldEqual, "bcrypt-hash")

		got, err = testUserStore.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(got.Nickname, ShouldEqual, "renamed")
	})
}

func TestUserStoreUpdateDatabaseErrors(t *testing.T) {
	Convey("UserStore update wraps database errors as ErrDatabase", t, func() {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		id := valobj.NewUid()
		now := time.Now().UnixMilli()

		So(pkgerrors.Is(testUserStore.UpdateProfile(ctx, &schema.UserUpdateProfileParams{
			Id: id, Nickname: "nick", UpdatedAt: now,
		}), pkgerrors.ErrDatabase), ShouldBeTrue)

		So(pkgerrors.Is(testUserStore.UpdateAvatar(ctx, &schema.UserUpdateAvatarParams{
			Id: id, Avatar: "stkv1-avatar", UpdatedAt: now,
		}), pkgerrors.ErrDatabase), ShouldBeTrue)

		So(pkgerrors.Is(testUserStore.UpdatePasswordHash(ctx, &schema.UserUpdatePasswordParams{
			Id: id, PasswordHash: "bcrypt-hash", UpdatedAt: now,
		}), pkgerrors.ErrDatabase), ShouldBeTrue)
	})
}
