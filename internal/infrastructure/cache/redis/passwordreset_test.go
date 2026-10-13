package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

// pending 里只放 token 哈希，查询时用邮件里的原始 token
func newTestPasswordResetPending(linkToken string) *schema.PasswordResetPending {
	return &schema.PasswordResetPending{
		Email:         uuid.NewV7().String() + "@example.com",
		LinkTokenHash: secretHash(linkToken),
		CreatedAt:     time.Now().UnixMilli(),
	}
}

func TestPasswordResetCacheSetGetByEmail(t *testing.T) {
	Convey("PasswordResetCache stores and reads a pending by email", t, func() {
		ctx := t.Context()
		pending := newTestPasswordResetPending("link-token-1")

		_, err := testPasswordResetCache.GetByEmail(ctx, pending.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		So(testPasswordResetCache.Set(ctx, pending, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testPasswordResetCache.DeleteByEmail(ctx, pending.Email) })

		got, err := testPasswordResetCache.GetByEmail(ctx, pending.Email)
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, pending.Email)
		So(got.LinkTokenHash, ShouldEqual, pending.LinkTokenHash)
		So(got.CreatedAt, ShouldEqual, pending.CreatedAt)
	})
}

func TestPasswordResetCacheLinkIndex(t *testing.T) {
	Convey("PasswordResetCache resolves the pending from the plaintext link token", t, func() {
		ctx := t.Context()
		pending := newTestPasswordResetPending("link-token-1")
		So(testPasswordResetCache.Set(ctx, pending, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testPasswordResetCache.DeleteByEmail(ctx, pending.Email) })

		got, err := testPasswordResetCache.GetByLinkToken(ctx, "link-token-1")
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, pending.Email)

		// 原始 token 要真能查到，哈希推导与写入侧一致
		_, err = testPasswordResetCache.GetByLinkToken(ctx, "link-token-unknown")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		// 重发后旧链接立即失效
		previous := pending.LinkTokenHash
		pending.LinkTokenHash = secretHash("link-token-2")
		So(testPasswordResetCache.Set(ctx, pending, previous, time.Hour), ShouldBeNil)

		_, err = testPasswordResetCache.GetByLinkToken(ctx, "link-token-1")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		got, err = testPasswordResetCache.GetByLinkToken(ctx, "link-token-2")
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, pending.Email)
	})
}

// 两个流程各用一套 key，同一邮箱的两条 pending 不能互相覆盖
func TestPasswordResetCacheIsolatedFromSignupKeys(t *testing.T) {
	Convey("PasswordResetCache does not share keys with SignupCache", t, func() {
		ctx := t.Context()
		email := uuid.NewV7().String() + "@example.com"

		signupPending := newTestSignupPending()
		signupPending.Email = email
		So(testSignupCache.Set(ctx, signupPending, time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testSignupCache.DeleteByEmail(ctx, email) })

		resetPending := newTestPasswordResetPending("reset-link-token")
		resetPending.Email = email
		So(testPasswordResetCache.Set(ctx, resetPending, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testPasswordResetCache.DeleteByEmail(ctx, email) })

		// 重置 pending 只删自己的记录，注册 pending 必须还在
		So(testPasswordResetCache.DeleteByEmail(ctx, email), ShouldBeNil)

		got, err := testSignupCache.GetByEmail(ctx, email)
		So(err, ShouldBeNil)
		So(got.TicketHash, ShouldEqual, signupPending.TicketHash)

		// 两条链接索引互不干扰
		_, err = testPasswordResetCache.GetByLinkToken(ctx, "reset-link-token")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestPasswordResetCacheCooldown(t *testing.T) {
	Convey("PasswordResetCache sets the cooldown marker only once", t, func() {
		ctx := t.Context()
		email := uuid.NewV7().String() + "@example.com"
		t.Cleanup(func() { _ = testPasswordResetCache.DeleteCooldown(ctx, email) })

		ok, err := testPasswordResetCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)

		// 已存在时不覆盖
		ok, err = testPasswordResetCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeFalse)

		// 删除后可再次写入
		So(testPasswordResetCache.DeleteCooldown(ctx, email), ShouldBeNil)
		ok, err = testPasswordResetCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
	})
}

// 重置密码邮件与验证邮件用同一邮箱时，冷却位必须各自独立，否则一个流程会挡住另一个
func TestPasswordResetCacheCooldownIsolatedFromSignup(t *testing.T) {
	Convey("PasswordResetCache cooldown does not block the signup cooldown", t, func() {
		ctx := t.Context()
		email := uuid.NewV7().String() + "@example.com"
		t.Cleanup(func() {
			_ = testPasswordResetCache.DeleteCooldown(ctx, email)
			_ = testSignupCache.DeleteCooldown(ctx, email)
		})

		ok, err := testSignupCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)

		// 注册发过信之后，找回密码仍应能发信
		ok, err = testPasswordResetCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
	})
}

func TestPasswordResetCacheCancelledContext(t *testing.T) {
	Convey("PasswordResetCache wraps redis failures as ErrCache", t, func() {
		ctx := canceledCtx(t)
		pending := newTestPasswordResetPending("link-token-cancelled")

		So(pkgerrors.Is(testPasswordResetCache.Set(ctx, pending, "", time.Hour), pkgerrors.ErrCache), ShouldBeTrue)

		_, err := testPasswordResetCache.GetByEmail(ctx, pending.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testPasswordResetCache.GetByLinkToken(ctx, "link-token-cancelled")
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testPasswordResetCache.SetCooldown(ctx, pending.Email, time.Minute)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		So(pkgerrors.Is(testPasswordResetCache.DeleteByEmail(ctx, pending.Email), pkgerrors.ErrCache), ShouldBeTrue)
		So(pkgerrors.Is(testPasswordResetCache.DeleteCooldown(ctx, pending.Email), pkgerrors.ErrCache), ShouldBeTrue)
	})
}

func TestPasswordResetCacheCorruptedPayload(t *testing.T) {
	Convey("PasswordResetCache rejects a corrupted payload with ErrSerde", t, func() {
		ctx := t.Context()
		impl := testPasswordResetCache.(*PasswordResetCacheImpl)
		pending := newTestPasswordResetPending("link-token-corrupted")
		key := impl.pendingKey(pending.Email)
		t.Cleanup(func() { _ = testRedis.Del(ctx, key).Err() })

		So(testRedis.Set(ctx, key, []byte{0xc1}, time.Minute).Err(), ShouldBeNil)

		_, err := testPasswordResetCache.GetByEmail(ctx, pending.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrSerde), ShouldBeTrue)
	})
}
