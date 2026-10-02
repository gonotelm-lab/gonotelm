package mapper

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
)

func UserSessionToSchema(s *identityentity.UserSession) *schema.UserSession {
	return &schema.UserSession{
		UserId:    s.UserId.String(),
		CreatedAt: s.CreatedAt.Value(),
		ExpireAt:  s.CreatedAt.Time().Add(s.Expiration).UnixMilli(),
		Device:    string(s.Device),
	}
}

func UserSessionFromSchema(id string, s *schema.UserSession) (*identityentity.UserSession, error) {
	userId, err := valobj.NewUidFromString(s.UserId)
	if err != nil {
		return nil, err
	}

	return &identityentity.UserSession{
		Id:         id,
		UserId:     userId,
		CreatedAt:  valobj.NewTimeFrom(s.CreatedAt),
		Expiration: time.Duration(s.ExpireAt-s.CreatedAt) * time.Millisecond,
		Device:     identityentity.DeviceType(s.Device),
	}, nil
}
