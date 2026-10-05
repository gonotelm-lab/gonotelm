package postgres

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
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

// Upsert 在记录不存在时必须退化成插入，不能只更新已有行。
func TestUserStoreUpsertInsertsWhenMissing(t *testing.T) {
	Convey("UserStore upsert inserts when missing", t, func() {
		store := testUserStore
		ctx := t.Context()

		user := &schema.User{
			Id:        valobj.NewUid(),
			Email:     uuid.NewV7().String() + "@example.com",
			Nickname:  "inserted_by_upsert",
			Status:    "active",
			Avatar:    "https://example.com/upsert.png",
			Provider:  "google",
			Sub:       uuid.NewV7().String(),
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		}

		err := store.Upsert(ctx, user)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", user.Id).Delete(&schema.User{}).Error
		})

		got, err := store.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(got.Nickname, ShouldEqual, "inserted_by_upsert")
		So(got.Avatar, ShouldEqual, "https://example.com/upsert.png")
		So(got.Provider, ShouldEqual, "google")

		// 再 upsert 一次走冲突更新分支：非冲突字段（provider/sub）保持不变。
		user.Nickname = "upsert_updated"
		user.Provider = "github"
		user.Sub = "changed-sub"
		user.UpdatedAt++
		So(store.Upsert(ctx, user), ShouldBeNil)

		gotAfter, err := store.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(gotAfter.Nickname, ShouldEqual, "upsert_updated")
		So(gotAfter.Provider, ShouldEqual, "google")
		So(gotAfter.Sub, ShouldNotEqual, "changed-sub")
	})
}

// 重复主键插入要被包装成数据库错误，而不是泄漏 gorm 原始错误。
func TestUserStoreCreateDuplicateIdFails(t *testing.T) {
	Convey("UserStore create duplicate id fails", t, func() {
		store := testUserStore
		ctx := t.Context()

		user := &schema.User{
			Id:        valobj.NewUid(),
			Email:     uuid.NewV7().String() + "@example.com",
			Nickname:  "dup",
			Status:    "active",
			Provider:  "github",
			Sub:       uuid.NewV7().String(),
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		}
		So(store.Create(ctx, user), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", user.Id).Delete(&schema.User{}).Error
		})

		err := store.Create(ctx, user)
		So(err, ShouldNotBeNil)
		So(pkgerrors.Is(err, pkgerrors.ErrDatabase), ShouldBeTrue)
	})
}

// (provider, sub) 唯一约束冲突同样要走错误包装分支。
func TestUserStoreCreateDuplicateProviderSubFails(t *testing.T) {
	Convey("UserStore create duplicate provider/sub fails", t, func() {
		store := testUserStore
		ctx := t.Context()

		sub := uuid.NewV7().String()
		first := &schema.User{
			Id:        valobj.NewUid(),
			Email:     uuid.NewV7().String() + "@example.com",
			Nickname:  "first",
			Status:    "active",
			Provider:  "github",
			Sub:       sub,
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		}
		So(store.Create(ctx, first), ShouldBeNil)

		second := &schema.User{
			Id:        valobj.NewUid(),
			Email:     uuid.NewV7().String() + "@example.com",
			Nickname:  "second",
			Status:    "active",
			Provider:  "github",
			Sub:       sub,
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		}
		err := store.Create(ctx, second)
		So(err, ShouldNotBeNil)
		So(pkgerrors.Is(err, pkgerrors.ErrDatabase), ShouldBeTrue)

		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("provider = ? AND sub = ?", "github", sub).Delete(&schema.User{}).Error
		})
	})
}

// 查不到记录时统一返回 ErrNoRecord，方便上层区分“不存在”与“查询失败”。
func TestUserStoreGetNotFoundReturnsNoRecord(t *testing.T) {
	Convey("UserStore get not found returns ErrNoRecord", t, func() {
		store := testUserStore
		ctx := t.Context()

		_, err := store.GetById(ctx, valobj.NewUid())
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		_, err = store.GetByProviderAndSub(ctx, "github", uuid.NewV7().String())
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)
	})
}

// 冲突目标只有 id，因此 (provider, sub) 撞唯一约束时仍会走插入失败分支。
func TestUserStoreUpsertProviderSubConflictFails(t *testing.T) {
	Convey("UserStore upsert provider/sub conflict fails", t, func() {
		store := testUserStore
		ctx := t.Context()

		sub := uuid.NewV7().String()
		first := &schema.User{
			Id:        valobj.NewUid(),
			Email:     uuid.NewV7().String() + "@example.com",
			Nickname:  "first",
			Status:    "active",
			Provider:  "github",
			Sub:       sub,
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		}
		So(store.Create(ctx, first), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("provider = ? AND sub = ?", "github", sub).Delete(&schema.User{}).Error
		})

		// 不同的 id、相同的 (provider, sub)：OnConflict 只针对 id，插入必然失败。
		conflict := &schema.User{
			Id:        valobj.NewUid(),
			Email:     uuid.NewV7().String() + "@example.com",
			Nickname:  "conflict",
			Status:    "active",
			Provider:  "github",
			Sub:       sub,
			CreatedAt: time.Now().UnixMilli(),
			UpdatedAt: time.Now().UnixMilli(),
		}
		err := store.Upsert(ctx, conflict)
		So(err, ShouldNotBeNil)
		So(pkgerrors.Is(err, pkgerrors.ErrDatabase), ShouldBeTrue)

		// 失败后不应留下半截数据。
		_, err = store.GetById(ctx, conflict.Id)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)
	})
}
