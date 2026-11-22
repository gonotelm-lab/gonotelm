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
		HasPassword:     view.HasPassword,
		Identities:      ToProtoIdentities(view.Identities),
	}
}

func ToProtoIdentities(views []*userapp.IdentityView) []*identityv1.Identity {
	if len(views) == 0 {
		return nil
	}

	out := make([]*identityv1.Identity, 0, len(views))
	for _, view := range views {
		if view == nil {
			continue
		}
		out = append(out, &identityv1.Identity{
			Id:                    view.Id,
			Provider:              ToProtoProvider(view.Provider),
			Subject:               view.Subject,
			ProviderEmail:         view.ProviderEmail,
			ProviderEmailVerified: view.ProviderEmailVerified,
			BoundAt:               TimeToProto(view.BoundAt),
			LastLoginAt:           TimeToProto(view.LastLoginAt),
		})
	}

	return out
}

func ToProtoProvider(provider entity.ProviderType) identityv1.Identity_Provider {
	switch provider {
	case entity.ProviderTypeGithub:
		return identityv1.Identity_PROVIDER_GITHUB
	case entity.ProviderTypeGoogle:
		return identityv1.Identity_PROVIDER_GOOGLE
	default:
		return identityv1.Identity_PROVIDER_UNSPECIFIED
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
