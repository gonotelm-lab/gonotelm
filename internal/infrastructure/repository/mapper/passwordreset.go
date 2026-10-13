package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
)

func PasswordResetToSchema(p *identityentity.PasswordResetPending) *schema.PasswordResetPending {
	return &schema.PasswordResetPending{
		Email:         p.Email,
		LinkTokenHash: p.LinkTokenHash,
		CreatedAt:     p.CreatedAt.Value(),
	}
}

func PasswordResetFromSchema(p *schema.PasswordResetPending) *identityentity.PasswordResetPending {
	return &identityentity.PasswordResetPending{
		Email:         p.Email,
		LinkTokenHash: p.LinkTokenHash,
		CreatedAt:     valobj.NewTimeFrom(p.CreatedAt),
	}
}
