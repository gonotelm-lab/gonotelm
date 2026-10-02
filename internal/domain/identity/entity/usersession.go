package entity

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const (
	// UserSessionIdleTimeout 闲置超时:距上次活跃超过该时长会话失效。
	// 活跃时通过 Touch 滑动续期,用户无感。
	UserSessionIdleTimeout = time.Hour * 72

	// UserSessionMaxAge 绝对上限:自登录起无论多活跃,超过该时长强制重新登录。
	UserSessionMaxAge = time.Hour * 24 * 60

	sessionIdLength = 32
)

type UserSession struct {
	Id        string
	UserId    valobj.Uid
	CreatedAt valobj.Time
	// ExpireAt 当前闲置过期时间点;活跃时滑动前移,但不越过绝对上限。
	ExpireAt valobj.Time
	Device   DeviceType
}

func NewUserSession(user *User, device DeviceType) (*UserSession, error) {
	id, err := idp.RandomString(sessionIdLength)
	if err != nil {
		return nil, err
	}

	createdAt := valobj.NewTime()
	s := &UserSession{
		Id:        id,
		UserId:    user.Id,
		CreatedAt: createdAt,
		ExpireAt:  valobj.NewTimeFrom(createdAt.Time().Add(UserSessionIdleTimeout).UnixMilli()),
		Device:    device,
	}
	return s, nil
}

// AbsoluteExpireAt 自登录起的绝对过期时间点,任何续期都不能越过它。
func (s *UserSession) AbsoluteExpireAt() time.Time {
	return s.CreatedAt.Time().Add(UserSessionMaxAge)
}

// Touch 滑动续期:把闲置过期时间前移到 now+UserSessionIdleTimeout,不越过绝对上限。
// 为避免每次请求都写存储,仅当剩余有效期不足闲置超时的一半时才真正续期。
// 返回记录是否发生变化(调用方才需要持久化)。
func (s *UserSession) Touch(now time.Time) bool {
	next := now.Add(UserSessionIdleTimeout)
	if deadline := s.AbsoluteExpireAt(); next.After(deadline) {
		next = deadline
	}

	if !next.After(s.ExpireAt.Time()) {
		// 已到绝对上限,或剩余有效期足够,无需续期
		return false
	}
	if s.ExpireAt.Time().Sub(now) > UserSessionIdleTimeout/2 {
		return false
	}

	s.ExpireAt = valobj.NewTimeFrom(next.UnixMilli())
	return true
}

func (s *UserSession) IsExpired(now time.Time) bool {
	return now.After(s.ExpireAt.Time()) || now.After(s.AbsoluteExpireAt())
}
