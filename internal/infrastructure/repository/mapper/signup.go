package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
)

func SignupToSchema(p *identityentity.SignupPending) *schema.SignupPending {
	return &schema.SignupPending{
		Email:         p.Email,
		PasswordHash:  p.PasswordHash,
		TicketHash:    p.TicketHash,
		LinkTokenHash: p.LinkTokenHash,
		CodeHash:      p.CodeHash,
		CodeExpireAt:  p.CodeExpireAt.Value(),
		CreatedAt:     p.CreatedAt.Value(),
	}
}

func SignupFromSchema(p *schema.SignupPending) *identityentity.SignupPending {
	return &identityentity.SignupPending{
		Email:         p.Email,
		PasswordHash:  p.PasswordHash,
		TicketHash:    p.TicketHash,
		LinkTokenHash: p.LinkTokenHash,
		CodeHash:      p.CodeHash,
		CodeExpireAt:  valobj.NewTimeFrom(p.CodeExpireAt),
		CreatedAt:     valobj.NewTimeFrom(p.CreatedAt),
	}
}
