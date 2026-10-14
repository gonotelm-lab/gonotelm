package redis

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

// state 里只放验证码哈希，匹配时用邮件里的原始验证码
func secretHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}

func newTestSignupState() *schema.SignupState {
	now := time.Now().UnixMilli()

	return &schema.SignupState{
		Email:        uuid.NewV7().String() + "@example.com",
		PasswordHash: "bcrypt-hash",
		TicketHash:   secretHash("ticket"),
		CodeHash:     secretHash("123456"),
		CodeExpireAt: now + int64(15*time.Minute/time.Millisecond),
		CreatedAt:    now,
	}
}

func TestSignupCacheSetGetByEmail(t *testing.T) {
	Convey("SignupCache stores and reads a state by email", t, func() {
		ctx := t.Context()
		state := newTestSignupState()

		_, err := testSignupCache.GetByEmail(ctx, state.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		So(testSignupCache.Set(ctx, state, time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testSignupCache.DeleteByEmail(ctx, state.Email) })

		got, err := testSignupCache.GetByEmail(ctx, state.Email)
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, state.Email)
		So(got.PasswordHash, ShouldEqual, "bcrypt-hash")
		So(got.TicketHash, ShouldEqual, state.TicketHash)
		So(got.CodeHash, ShouldEqual, state.CodeHash)
		So(got.CodeExpireAt, ShouldEqual, state.CodeExpireAt)
	})
}

func TestSignupCacheIncrAttempts(t *testing.T) {
	Convey("SignupCache increments code attempts atomically", t, func() {
		ctx := t.Context()
		state := newTestSignupState()
		So(testSignupCache.Set(ctx, state, time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testSignupCache.DeleteByEmail(ctx, state.Email) })

		for i := int64(1); i <= 3; i++ {
			attempts, err := testSignupCache.IncrAttempts(ctx, state.Email, time.Hour)
			So(err, ShouldBeNil)
			So(attempts, ShouldEqual, i)
		}
	})
}

func TestSignupCacheDeleteRemovesAttempts(t *testing.T) {
	Convey("SignupCache delete drops the state and its attempt counter", t, func() {
		ctx := t.Context()
		state := newTestSignupState()
		So(testSignupCache.Set(ctx, state, time.Hour), ShouldBeNil)

		_, err := testSignupCache.IncrAttempts(ctx, state.Email, time.Hour)
		So(err, ShouldBeNil)

		So(testSignupCache.DeleteByEmail(ctx, state.Email), ShouldBeNil)

		_, err = testSignupCache.GetByEmail(ctx, state.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		attempts, err := testSignupCache.IncrAttempts(ctx, state.Email, time.Hour)
		So(err, ShouldBeNil)
		So(attempts, ShouldEqual, 1)
	})
}

func TestSignupCacheCooldown(t *testing.T) {
	Convey("SignupCache sets the cooldown marker only once", t, func() {
		ctx := t.Context()
		email := uuid.NewV7().String() + "@example.com"
		t.Cleanup(func() { _ = testSignupCache.DeleteCooldown(ctx, email) })

		ok, err := testSignupCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)

		// 已存在时不覆盖
		ok, err = testSignupCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeFalse)

		// 删除后可再次写入
		So(testSignupCache.DeleteCooldown(ctx, email), ShouldBeNil)
		ok, err = testSignupCache.SetCooldown(ctx, email, time.Minute)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
	})
}

func TestSignupCacheCancelledContext(t *testing.T) {
	Convey("SignupCache wraps redis failures as ErrCache", t, func() {
		ctx := canceledCtx(t)
		state := newTestSignupState()

		So(pkgerrors.Is(testSignupCache.Set(ctx, state, time.Hour), pkgerrors.ErrCache), ShouldBeTrue)

		_, err := testSignupCache.GetByEmail(ctx, state.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testSignupCache.IncrAttempts(ctx, state.Email, time.Hour)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testSignupCache.SetCooldown(ctx, state.Email, time.Minute)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		So(pkgerrors.Is(testSignupCache.DeleteByEmail(ctx, state.Email), pkgerrors.ErrCache), ShouldBeTrue)
		So(pkgerrors.Is(testSignupCache.DeleteCooldown(ctx, state.Email), pkgerrors.ErrCache), ShouldBeTrue)
	})
}

func TestSignupCacheCorruptedPayload(t *testing.T) {
	Convey("SignupCache rejects a corrupted payload with ErrSerde", t, func() {
		ctx := t.Context()
		impl := testSignupCache.(*SignupCacheImpl)
		state := newTestSignupState()
		key := impl.stateKey(state.Email)
		t.Cleanup(func() { _ = testRedis.Del(ctx, key).Err() })

		So(testRedis.Set(ctx, key, []byte{0xc1}, time.Minute).Err(), ShouldBeNil)

		_, err := testSignupCache.GetByEmail(ctx, state.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrSerde), ShouldBeTrue)
	})
}
