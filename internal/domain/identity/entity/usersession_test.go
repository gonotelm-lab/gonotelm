package entity

import (
	"bytes"
	"encoding/base64"
	"strings"
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

func TestNewUserSessionId(t *testing.T) {
	user := NewUser("tester@example.com", "tester", valobj.NewTime())
	s, err := NewUserSession(user, DeviceTypeWeb, LoginSourcePassword)
	if err != nil {
		t.Fatalf("NewUserSession failed: %v", err)
	}
	if !s.UserId.EqualsTo(user.Id) {
		t.Fatal("session must carry the user id")
	}

	t.Log(s.Id)

	parts := strings.Split(s.Id, sessionIdSeparator)
	if len(parts) != 2 {
		t.Fatalf("unexpected session id shape: got %q want <random>%s<account>", s.Id, sessionIdSeparator)
	}

	random, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("random segment is not base64url: %v", err)
	}
	if len(random) != sessionIdLength {
		t.Fatalf("unexpected random segment length: got %d want %d", len(random), sessionIdLength)
	}

	account, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("account segment is not base64url: %v", err)
	}

	want := make([]byte, 16)
	copy(want, user.Id.Bytes())
	for i := 0; i < userIdSegmentSize; i++ {
		want[i], want[2*userIdSegmentSize+i] = want[2*userIdSegmentSize+i], want[i]
	}
	if !bytes.Equal(account, want) {
		t.Fatalf("unexpected account segment: got %x want %x", account, want)
	}
}

func TestNewUserSessionIdAccountSegmentStable(t *testing.T) {
	user := NewUser("tester@example.com", "tester", valobj.NewTime())

	first, err := NewUserSession(user, DeviceTypeWeb, LoginSourcePassword)
	if err != nil {
		t.Fatalf("NewUserSession failed: %v", err)
	}
	second, err := NewUserSession(user, DeviceTypeWeb, LoginSourcePassword)
	if err != nil {
		t.Fatalf("NewUserSession failed: %v", err)
	}

	if first.Id == second.Id {
		t.Fatal("session ids of the same user must differ in the random segment")
	}
	firstParts := strings.Split(first.Id, sessionIdSeparator)
	secondParts := strings.Split(second.Id, sessionIdSeparator)
	if firstParts[1] != secondParts[1] {
		t.Fatalf("account segment must be stable for the same user: got %q and %q", firstParts[1], secondParts[1])
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

	// 绝对上限优先：ExpireAt 设得再远，过了上限也算过期
	s.ExpireAt = valobj.NewTimeFrom(now.Add(UserSessionMaxAge * 2).UnixMilli())
	if !s.IsExpired(s.AbsoluteExpireAt().Add(time.Second)) {
		t.Fatal("session must expire at the absolute cap")
	}
}
