package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func UserIdentityToSchema(i *identityentity.UserIdentity) *schema.UserIdentity {
	return &schema.UserIdentity{
		Id:                    i.Id,
		UserId:                i.UserId,
		Provider:              i.Provider.String(),
		Subject:               i.Subject,
		ProviderEmail:         i.ProviderEmail,
		ProviderEmailVerified: i.ProviderEmailVerified,
		LastLoginAt:           i.LastLoginAt.Value(),
		CreatedAt:             i.CreatedAt.Value(),
		UpdatedAt:             i.UpdatedAt.Value(),
	}
}

func UserIdentityFromSchema(i *schema.UserIdentity) *identityentity.UserIdentity {
	return &identityentity.UserIdentity{
		Id:                    i.Id,
		UserId:                i.UserId,
		Provider:              identityentity.ProviderType(i.Provider),
		Subject:               i.Subject,
		ProviderEmail:         i.ProviderEmail,
		ProviderEmailVerified: i.ProviderEmailVerified,
		LastLoginAt:           valobj.NewTimeFrom(i.LastLoginAt),
		CreatedAt:             valobj.NewTimeFrom(i.CreatedAt),
		UpdatedAt:             valobj.NewTimeFrom(i.UpdatedAt),
	}
}
