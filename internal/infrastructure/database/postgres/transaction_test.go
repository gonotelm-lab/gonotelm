package postgres

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

// 事务内的 store 调用必须并入同一事务：这里用"跨表写入 + 中途失败"验证回滚。
func TestTransactorCommitsAllWrites(t *testing.T) {
	Convey("Transactor commits every store call inside the closure", t, func() {
		ctx := t.Context()
		user := newTestUserRow("")
		identity := newTestIdentityRow(user.Id)
		cleanupTestUser(t, user.Id)
		cleanupTestIdentity(t, identity.Id)

		err := testTransactor.Transaction(ctx, func(txCtx context.Context) error {
			if err := testUserStore.Create(txCtx, user); err != nil {
				return err
			}

			return testUserIdentityStore.Create(txCtx, identity)
		})
		So(err, ShouldBeNil)

		_, err = testUserStore.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		_, err = testUserIdentityStore.GetByProviderAndSubject(ctx, identity.Provider, identity.Subject)
		So(err, ShouldBeNil)
	})
}

func TestTransactorRollsBackOnError(t *testing.T) {
	Convey("Transactor rolls back every write when the closure fails", t, func() {
		ctx := t.Context()
		user := newTestUserRow("")
		cleanupTestUser(t, user.Id)

		// 先占位一个 provider+subject，让闭包里的第二次写入撞唯一索引
		conflict := newTestIdentityRow(valobj.NewUid())
		cleanupTestIdentity(t, conflict.Id)
		So(testUserIdentityStore.Create(ctx, conflict), ShouldBeNil)

		dup := newTestIdentityRow(user.Id)
		dup.Provider = conflict.Provider
		dup.Subject = conflict.Subject

		err := testTransactor.Transaction(ctx, func(txCtx context.Context) error {
			if err := testUserStore.Create(txCtx, user); err != nil {
				return err
			}

			return testUserIdentityStore.Create(txCtx, dup)
		})
		So(err, ShouldNotBeNil)

		// 事务内先写入的账号必须一起回滚
		_, err = testUserStore.GetById(ctx, user.Id)
		So(err, ShouldNotBeNil)
	})
}

func TestTransactorWithoutTxUsesPool(t *testing.T) {
	Convey("Store calls outside a transaction still work", t, func() {
		ctx := t.Context()
		user := newTestUserRow("")
		cleanupTestUser(t, user.Id)

		So(testUserStore.Create(ctx, user), ShouldBeNil)

		got, err := testUserStore.GetById(ctx, user.Id)
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, user.Email)
	})
}

func TestBuildIdentitySubjectUniqueInsideTx(t *testing.T) {
	Convey("A rolled back transaction leaves no identity behind", t, func() {
		ctx := t.Context()
		subject := uuid.NewV7().String()
		userId := valobj.NewUid()

		err := testTransactor.Transaction(ctx, func(txCtx context.Context) error {
			identity := newTestIdentityRow(userId)
			identity.Subject = subject
			if err := testUserIdentityStore.Create(txCtx, identity); err != nil {
				return err
			}

			return context.Canceled
		})
		So(err, ShouldNotBeNil)

		_, err = testUserIdentityStore.GetByProviderAndSubject(ctx, "github", subject)
		So(err, ShouldNotBeNil)
	})
}
