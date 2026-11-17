package schema

import (
	userapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/user"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"

	identityv1 "github.com/gonotelm-lab/apis-go-client/apps/notelm/dev/identity/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func ToProtoUser(view *userapp.UserView) *identityv1.User {
	if view == nil {
		return nil
	}

	return &identityv1.User{
		Id:              view.Id,
		Nickname:        view.Nickname,
		EmailVerifiedAt: TimeToProto(view.EmailVerifiedAt),
		Status:          ToProtoStatus(view.Status),
		Avatar:          view.AvatarURL,
		CreatedAt:       TimeToProto(view.CreatedAt),
		UpdatedAt:       TimeToProto(view.UpdatedAt),
	}
}

func ToProtoStatus(status entity.UserStatus) identityv1.User_Status {
	switch status {
	case entity.UserStatusActive:
		return identityv1.User_STATUS_ACTIVE
	case entity.UserStatusBanned:
		return identityv1.User_STATUS_BANNED
	default:
		return identityv1.User_STATUS_UNSPECIFIED
	}
}

func FromProtoStatus(status identityv1.User_Status) (entity.UserStatus, error) {
	switch status {
	case identityv1.User_STATUS_ACTIVE:
		return entity.UserStatusActive, nil
	case identityv1.User_STATUS_BANNED:
		return entity.UserStatusBanned, nil
	default:
		return "", pkgerrors.ErrParams.Msg("invalid user status")
	}
}

func TimeToProto(t valobj.Time) *timestamppb.Timestamp {
	if t.Value() <= 0 {
		return nil
	}
	return timestamppb.New(t.Time())
}
