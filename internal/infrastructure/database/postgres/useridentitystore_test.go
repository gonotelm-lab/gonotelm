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

func newTestIdentityRow(userId valobj.Uid) *schema.UserIdentity {
	now := time.Now().UnixMilli()

	return &schema.UserIdentity{
		Id:                    valobj.NewId(),
		UserId:                userId,
		Provider:              "github",
		Subject:               uuid.NewV7().String(),
		ProviderEmail:         "nick@example.com",
		ProviderEmailVerified: true,
		LastLoginAt:           now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

func cleanupTestIdentity(t *testing.T, id valobj.Id) {
	t.Helper()
	t.Cleanup(func() {
		_ = testDB.Where("id = ?", id).Delete(&schema.UserIdentity{}).Error
	})
}

func TestUserIdentityStoreCreateAndRead(t *testing.T) {
	Convey("UserIdentityStore Create then read back by provider+subject", t, func() {
		ctx := t.Context()
		userId := valobj.NewUid()
		identity := newTestIdentityRow(userId)
		cleanupTestIdentity(t, identity.Id)

		So(testUserIdentityStore.Create(ctx, identity), ShouldBeNil)

		got, err := testUserIdentityStore.GetByProviderAndSubject(ctx, identity.Provider, identity.Subject)
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, identity.Id)
		So(got.UserId, ShouldEqual, userId)
		So(got.ProviderEmailVerified, ShouldBeTrue)

		list, err := testUserIdentityStore.ListByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(len(list), ShouldEqual, 1)
		So(list[0].Id, ShouldEqual, identity.Id)
	})
}

func TestUserIdentityStoreProviderSubjectUnique(t *testing.T) {
	Convey("UserIdentityStore rejects a duplicate provider+subject", t, func() {
		ctx := t.Context()
		userId := valobj.NewUid()
		identity := newTestIdentityRow(userId)
		cleanupTestIdentity(t, identity.Id)
		So(testUserIdentityStore.Create(ctx, identity), ShouldBeNil)

		dup := newTestIdentityRow(valobj.NewUid())
		dup.Provider = identity.Provider
		dup.Subject = identity.Subject
		So(testUserIdentityStore.Create(ctx, dup), ShouldNotBeNil)
	})
}

// uk_user_identities_v2_user_provider：一个账号在每个 provider 上最多一个身份
// 换 provider 必须放行，否则一个账号绑 github + google 会被误伤
func TestUserIdentityStoreUserProviderUnique(t *testing.T) {
	Convey("UserIdentityStore allows one identity per account and provider", t, func() {
		ctx := t.Context()
		userId := valobj.NewUid()

		first := newTestIdentityRow(userId)
		cleanupTestIdentity(t, first.Id)
		So(testUserIdentityStore.Create(ctx, first), ShouldBeNil)

		second := newTestIdentityRow(userId)
		So(testUserIdentityStore.Create(ctx, second), ShouldNotBeNil)

		third := newTestIdentityRow(userId)
		third.Provider = "google"
		cleanupTestIdentity(t, third.Id)
		So(testUserIdentityStore.Create(ctx, third), ShouldBeNil)

		list, err := testUserIdentityStore.ListByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(len(list), ShouldEqual, 2)
	})
}

func TestUserIdentityStoreDeleteOne(t *testing.T) {
	Convey("UserIdentityStore deletes a single identity and leaves the others", t, func() {
		ctx := t.Context()
		userId := valobj.NewUid()

		github := newTestIdentityRow(userId)
		cleanupTestIdentity(t, github.Id)
		So(testUserIdentityStore.Create(ctx, github), ShouldBeNil)

		google := newTestIdentityRow(userId)
		google.Provider = "google"
		cleanupTestIdentity(t, google.Id)
		So(testUserIdentityStore.Create(ctx, google), ShouldBeNil)

		So(testUserIdentityStore.Delete(ctx, github.Id), ShouldBeNil)

		list, err := testUserIdentityStore.ListByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(len(list), ShouldEqual, 1)
		So(list[0].Id, ShouldEqual, google.Id)

		// 重复删除不报错：解绑天然幂等
		So(testUserIdentityStore.Delete(ctx, github.Id), ShouldBeNil)
	})
}

// 归属由查询保证：别人的 bind_id 取不到
func TestUserIdentityStoreGetByUserIdAndId(t *testing.T) {
	Convey("UserIdentityStore scopes the lookup by owner", t, func() {
		ctx := t.Context()
		userId := valobj.NewUid()
		identity := newTestIdentityRow(userId)
		cleanupTestIdentity(t, identity.Id)
		So(testUserIdentityStore.Create(ctx, identity), ShouldBeNil)

		got, err := testUserIdentityStore.GetByUserIdAndId(ctx, userId, identity.Id)
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, identity.Id)

		// 同一个 id 换个账号：取不到
		_, err = testUserIdentityStore.GetByUserIdAndId(ctx, valobj.NewUid(), identity.Id)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		// 不存在的 id
		_, err = testUserIdentityStore.GetByUserIdAndId(ctx, userId, valobj.NewId())
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestUserIdentityStoreReadMissing(t *testing.T) {
	Convey("UserIdentityStore returns ErrNoRecord for unknown identity", t, func() {
		_, err := testUserIdentityStore.GetByProviderAndSubject(t.Context(), "github", "no-such-subject")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestUserIdentityStoreTouchAndDelete(t *testing.T) {
	Convey("UserIdentityStore touches login time and deletes by user", t, func() {
		ctx := t.Context()
		userId := valobj.NewUid()
		identity := newTestIdentityRow(userId)
		cleanupTestIdentity(t, identity.Id)
		So(testUserIdentityStore.Create(ctx, identity), ShouldBeNil)

		touched := time.Now().UnixMilli() + 1000
		So(testUserIdentityStore.UpdateLastLoginAt(ctx, identity.Id, touched), ShouldBeNil)

		got, err := testUserIdentityStore.GetByProviderAndSubject(ctx, identity.Provider, identity.Subject)
		So(err, ShouldBeNil)
		So(got.LastLoginAt, ShouldEqual, touched)

		So(testUserIdentityStore.DeleteByUserId(ctx, userId), ShouldBeNil)

		list, err := testUserIdentityStore.ListByUserId(ctx, userId)
		So(err, ShouldBeNil)
		So(len(list), ShouldEqual, 0)
	})
}

func TestUserIdentityStoreDatabaseErrors(t *testing.T) {
	Convey("UserIdentityStore wraps database errors as ErrDatabase", t, func() {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		userId := valobj.NewUid()
		identity := newTestIdentityRow(userId)

		So(pkgerrors.Is(testUserIdentityStore.Create(ctx, identity), pkgerrors.ErrDatabase), ShouldBeTrue)

		_, err := testUserIdentityStore.ListByUserId(ctx, userId)
		So(pkgerrors.Is(err, pkgerrors.ErrDatabase), ShouldBeTrue)

		So(pkgerrors.Is(
			testUserIdentityStore.UpdateLastLoginAt(ctx, identity.Id, time.Now().UnixMilli()),
			pkgerrors.ErrDatabase,
		), ShouldBeTrue)

		So(pkgerrors.Is(testUserIdentityStore.Delete(ctx, identity.Id), pkgerrors.ErrDatabase), ShouldBeTrue)
		So(pkgerrors.Is(testUserIdentityStore.DeleteByUserId(ctx, userId), pkgerrors.ErrDatabase), ShouldBeTrue)
	})
}
