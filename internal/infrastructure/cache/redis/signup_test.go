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

// pending 里只放 token 哈希，查询时用邮件里的原始 token
func secretHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}

func newTestSignupPending(linkToken string) *schema.SignupPending {
	now := time.Now().UnixMilli()

	return &schema.SignupPending{
		Email:         uuid.NewV7().String() + "@example.com",
		PasswordHash:  "bcrypt-hash",
		TicketHash:    secretHash("ticket"),
		LinkTokenHash: secretHash(linkToken),
		CodeHash:      secretHash("123456"),
		CodeExpireAt:  now + int64(15*time.Minute/time.Millisecond),
		CreatedAt:     now,
	}
}

func TestSignupCacheSetGetByEmail(t *testing.T) {
	Convey("SignupCache stores and reads a pending by email", t, func() {
		ctx := t.Context()
		pending := newTestSignupPending("link-token-1")

		_, err := testSignupCache.GetByEmail(ctx, pending.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		So(testSignupCache.Set(ctx, pending, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testSignupCache.DeleteByEmail(ctx, pending.Email) })

		got, err := testSignupCache.GetByEmail(ctx, pending.Email)
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, pending.Email)
		So(got.PasswordHash, ShouldEqual, "bcrypt-hash")
		So(got.TicketHash, ShouldEqual, pending.TicketHash)
		So(got.CodeExpireAt, ShouldEqual, pending.CodeExpireAt)
	})
}

func TestSignupCacheLinkIndex(t *testing.T) {
	Convey("SignupCache resolves the pending from the plaintext link token", t, func() {
		ctx := t.Context()
		pending := newTestSignupPending("link-token-1")
		So(testSignupCache.Set(ctx, pending, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testSignupCache.DeleteByEmail(ctx, pending.Email) })

		got, err := testSignupCache.GetByLinkToken(ctx, "link-token-1")
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, pending.Email)

		// 原始 token 要真能查到，哈希推导与写入侧一致
		_, err = testSignupCache.GetByLinkToken(ctx, "link-token-unknown")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		// 重发后旧链接立即失效
		previous := pending.LinkTokenHash
		pending.LinkTokenHash = secretHash("link-token-2")
		So(testSignupCache.Set(ctx, pending, previous, time.Hour), ShouldBeNil)

		_, err = testSignupCache.GetByLinkToken(ctx, "link-token-1")
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		got, err = testSignupCache.GetByLinkToken(ctx, "link-token-2")
		So(err, ShouldBeNil)
		So(got.Email, ShouldEqual, pending.Email)
	})
}

func TestSignupCacheIncrAttempts(t *testing.T) {
	Convey("SignupCache increments code attempts atomically", t, func() {
		ctx := t.Context()
		pending := newTestSignupPending("link-token-1")
		So(testSignupCache.Set(ctx, pending, "", time.Hour), ShouldBeNil)
		t.Cleanup(func() { _ = testSignupCache.DeleteByEmail(ctx, pending.Email) })

		for i := int64(1); i <= 3; i++ {
			attempts, err := testSignupCache.IncrAttempts(ctx, pending.Email, time.Hour)
			So(err, ShouldBeNil)
			So(attempts, ShouldEqual, i)
		}
	})
}

func TestSignupCacheDeleteRemovesAttempts(t *testing.T) {
	Convey("SignupCache delete drops the pending and its attempt counter", t, func() {
		ctx := t.Context()
		pending := newTestSignupPending("link-token-1")
		So(testSignupCache.Set(ctx, pending, "", time.Hour), ShouldBeNil)

		_, err := testSignupCache.IncrAttempts(ctx, pending.Email, time.Hour)
		So(err, ShouldBeNil)

		So(testSignupCache.DeleteByEmail(ctx, pending.Email), ShouldBeNil)

		_, err = testSignupCache.GetByEmail(ctx, pending.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)

		attempts, err := testSignupCache.IncrAttempts(ctx, pending.Email, time.Hour)
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
		pending := newTestSignupPending("link-token-cancelled")

		So(pkgerrors.Is(testSignupCache.Set(ctx, pending, "", time.Hour), pkgerrors.ErrCache), ShouldBeTrue)

		_, err := testSignupCache.GetByEmail(ctx, pending.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testSignupCache.GetByLinkToken(ctx, "link-token-cancelled")
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testSignupCache.IncrAttempts(ctx, pending.Email, time.Hour)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		_, err = testSignupCache.SetCooldown(ctx, pending.Email, time.Minute)
		So(pkgerrors.Is(err, pkgerrors.ErrCache), ShouldBeTrue)

		So(pkgerrors.Is(testSignupCache.DeleteByEmail(ctx, pending.Email), pkgerrors.ErrCache), ShouldBeTrue)
		So(pkgerrors.Is(testSignupCache.DeleteCooldown(ctx, pending.Email), pkgerrors.ErrCache), ShouldBeTrue)
	})
}

func TestSignupCacheCorruptedPayload(t *testing.T) {
	Convey("SignupCache rejects a corrupted payload with ErrSerde", t, func() {
		ctx := t.Context()
		impl := testSignupCache.(*SignupCacheImpl)
		pending := newTestSignupPending("link-token-corrupted")
		key := impl.pendingKey(pending.Email)
		t.Cleanup(func() { _ = testRedis.Del(ctx, key).Err() })

		So(testRedis.Set(ctx, key, []byte{0xc1}, time.Minute).Err(), ShouldBeNil)

		_, err := testSignupCache.GetByEmail(ctx, pending.Email)
		So(pkgerrors.Is(err, pkgerrors.ErrSerde), ShouldBeTrue)
	})
}
