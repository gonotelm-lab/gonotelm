package entity

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const (
	UserSessionExpiration = time.Hour * 48

	sessionIdLength = 32
)

type UserSession struct {
	Id         string
	UserId     valobj.Uid
	CreatedAt  valobj.Time
	Expiration time.Duration
	Device     DeviceType
}

func NewUserSession(user *User, device DeviceType) (*UserSession, error) {
	id, err := idp.RandomString(sessionIdLength)
	if err != nil {
		return nil, err
	}

	s := &UserSession{
		Id:         id,
		UserId:     user.Id,
		CreatedAt:  valobj.NewTime(),
		Expiration: UserSessionExpiration,
		Device:     device,
	}
	return s, nil
}

func (s *UserSession) IsExpired(now time.Time) bool {
	return now.After(s.CreatedAt.Time().Add(s.Expiration))
}
