package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

// state 里只放 token 哈希，查询时用邮件里的原始 token
func newTestPasswordResetState(linkToken string) *schema.PasswordResetState {
	return &schema.PasswordResetState{
		Email:         uuid.NewV7().String() + "@example.com",
		LinkTokenHash: secretHash(linkToken),
		CreatedAt:     time.Now().UnixMilli(),
	}
}

func TestPasswordResetCacheSetGetByEmail(t *testing.T) {
	Convey("PasswordResetCache stores and reads a state by email", t, func() {
		ctx := t.Context()
		state := newTestPasswordResetState("link-token-1")

		_, err := testPasswordResetCache.GetByEmail(ctx, state.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		So(testPasswordResetCache.Set(ctx, state, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testPasswordResetCache.DeleteByEmail(ctx, state.Email) })

		got, err := testPasswordResetCache.GetByEmail(ctx, state.Email)
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, state.Email)
		So(got.LinkTokenHash, ShouldEqual, state.LinkTokenHash)
		So(got.CreatedAt, ShouldEqual, state.CreatedAt)
	})
}

func TestPasswordResetCacheLinkIndex(t *testing.T) {
	Convey("PasswordResetCache resolves the state from the plaintext link token", t, func() {
		ctx := t.Context()
		state := newTestPasswordResetState("link-token-1")
		So(testPasswordResetCache.Set(ctx, state, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testPasswordResetCache.DeleteByEmail(ctx, state.Email) })

		got, err := testPasswordResetCache.GetByLinkToken(ctx, "link-token-1")
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, state.Email)

		// 原始 token 要真能查到，哈希推导与写入侧一致
		_, err = testPasswordResetCache.GetByLinkToken(ctx, "link-token-unknown")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		// 重发后旧链接立即失效
		previous := state.LinkTokenHash
		state.LinkTokenHash = secretHash("link-token-2")
		So(testPasswordResetCache.Set(ctx, state, previous, time.Hour), ShouldBeNil)

		_, err = testPasswordResetCache.GetByLinkToken(ctx, "link-token-1")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		got, err = testPasswordResetCache.GetByLinkToken(ctx, "link-token-2")
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, state.Email)
	})
}

// 两个流程各用一套 key，同一邮箱的两条 state 不能互相覆盖
func TestPasswordResetCacheIsolatedFromSignupKeys(t *testing.T) {
	Convey("PasswordResetCache does not share keys with SignupCache", t, func() {
		ctx := t.Context()
		email := uuid.NewV7().String() + "@example.com"

		signupState := newTestSignupState()
		signupState.Email = email
		So(testSignupCache.Set(ctx, signupState, time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testSignupCache.DeleteByEmail(ctx, email) })

		resetState := newTestPasswordResetState("reset-link-token")
		resetState.Email = email
		So(testPasswordResetCache.Set(ctx, resetState, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testPasswordResetCache.DeleteByEmail(ctx, email) })

		// 重置 state 只删自己的记录，注册 state 必须还在
		So(testPasswordResetCache.DeleteByEmail(ctx, email), ShouldBeNil)

		got, err := testSignupCache.GetByEmail(ctx, email)
		So(err, ShouldBeNil)
		So(got.TicketHash, ShouldEqual, signupState.TicketHash)

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

		// 注册发过邮件之后，找回密码仍应能发送邮件
		ok, err = testPasswordResetCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
	})
}

func TestPasswordResetCacheCancelledContext(t *testing.T) {
	Convey("PasswordResetCache wraps redis failures as ErrCache", t, func() {
		ctx := canceledCtx(t)
		state := newTestPasswordResetState("link-token-cancelled")

		So(pkgerrors.Is(testPasswordResetCache.Set(ctx, state, "", time.Hour), pkgerrors.ErrCache), ShouldBeTrue)

		_, err := testPasswordResetCache.GetByEmail(ctx, state.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testPasswordResetCache.GetByLinkToken(ctx, "link-token-cancelled")
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testPasswordResetCache.SetCooldown(ctx, state.Email, time.Minute)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		So(pkgerrors.Is(testPasswordResetCache.DeleteByEmail(ctx, state.Email), pkgerrors.ErrCache), ShouldBeTrue)
		So(pkgerrors.Is(testPasswordResetCache.DeleteCooldown(ctx, state.Email), pkgerrors.ErrCache), ShouldBeTrue)
	})
}

func TestPasswordResetCacheCorruptedPayload(t *testing.T) {
	Convey("PasswordResetCache rejects a corrupted payload with ErrSerde", t, func() {
		ctx := t.Context()
		impl := testPasswordResetCache.(*PasswordResetCacheImpl)
		state := newTestPasswordResetState("link-token-corrupted")
		key := impl.stateKey(state.Email)
		t.Cleanup(func() { _ = testRedis.Del(ctx, key).Err() })

		So(testRedis.Set(ctx, key, []byte{0xc1}, time.Minute).Err(), ShouldBeNil)

		_, err := testPasswordResetCache.GetByEmail(ctx, state.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrSerde), ShouldBeTrue)
	})
}
