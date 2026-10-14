package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
)

func PasswordResetToSchema(p *identityentity.PasswordResetState) *schema.PasswordResetState {
	return &schema.PasswordResetState{
		Email:         p.Email,
		LinkTokenHash: p.LinkTokenHash,
		CreatedAt:     p.CreatedAt.Value(),
	}
}

func PasswordResetFromSchema(p *schema.PasswordResetState) *identityentity.PasswordResetState {
	return &identityentity.PasswordResetState{
		Email:         p.Email,
		LinkTokenHash: p.LinkTokenHash,
		CreatedAt:     valobj.NewTimeFrom(p.CreatedAt),
	}
}
