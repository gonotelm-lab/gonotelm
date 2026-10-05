package entity

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

func newTestSession(t *testing.T, now time.Time) *UserSession {
	t.Helper()

	createdAt := valobj.NewTimeFrom(now.UnixMilli())
	return &UserSession{
		Id:        "test-session-id",
		UserId:    valobj.NewUid(),
		CreatedAt: createdAt,
		ExpireAt:  valobj.NewTimeFrom(now.Add(UserSessionIdleTimeout).UnixMilli()),
		Device:    DeviceTypeWeb,
	}
}

// assertExpireAt 比较到期时间点(valobj.Time 只精确到毫秒)
func assertExpireAt(t *testing.T, s *UserSession, want time.Time) {
	t.Helper()

	if got := s.ExpireAt.Value(); got != want.UnixMilli() {
		t.Fatalf("unexpected expire at: got %v want %v",
			time.UnixMilli(got), want.Truncate(time.Millisecond))
	}
}

func TestUserSessionTouchSliding(t *testing.T) {
	now := time.Now()
	s := newTestSession(t, now)

	// 闲置窗口还很充足:不续期
	if s.Touch(now.Add(time.Hour)) {
		t.Fatal("expected no renewal while more than half of the idle timeout remains")
	}
	assertExpireAt(t, s, now.Add(UserSessionIdleTimeout))

	// 剩余不足一半:滑动续期
	at := now.Add(UserSessionIdleTimeout/2 + time.Hour)
	if !s.Touch(at) {
		t.Fatal("expected renewal once less than half of the idle timeout remains")
	}
	assertExpireAt(t, s, at.Add(UserSessionIdleTimeout))
}

func TestUserSessionTouchAbsoluteCap(t *testing.T) {
	now := time.Now()
	s := newTestSession(t, now)

	// 接近绝对上限:续期被截断到上限
	at := s.AbsoluteExpireAt().Add(-time.Hour)
	if !s.Touch(at) {
		t.Fatal("expected renewal near the absolute cap")
	}
	assertExpireAt(t, s, s.AbsoluteExpireAt())

	// 已到绝对上限:不再续期
	if s.Touch(s.AbsoluteExpireAt().Add(-time.Minute)) {
		t.Fatal("expected no renewal past the absolute cap")
	}
	assertExpireAt(t, s, s.AbsoluteExpireAt())
}

func TestUserSessionIsExpired(t *testing.T) {
	now := time.Now()
	s := newTestSession(t, now)

	if s.IsExpired(now) {
		t.Fatal("fresh session must not be expired")
	}
	if !s.IsExpired(now.Add(UserSessionIdleTimeout + time.Second)) {
		t.Fatal("session must expire after the idle timeout")
	}

	// 绝对上限兜底:即使 ExpireAt 被错误地设得很远,过了上限也算过期
	s.ExpireAt = valobj.NewTimeFrom(now.Add(UserSessionMaxAge * 2).UnixMilli())
	if !s.IsExpired(s.AbsoluteExpireAt().Add(time.Second)) {
		t.Fatal("session must expire at the absolute cap")
	}
}
