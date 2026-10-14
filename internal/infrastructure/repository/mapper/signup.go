package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
)

func SignupToSchema(p *identityentity.SignupState) *schema.SignupState {
	return &schema.SignupState{
		Email:        p.Email,
		PasswordHash: p.PasswordHash,
		TicketHash:   p.TicketHash,
		CodeHash:     p.CodeHash,
		CodeExpireAt: p.CodeExpireAt.Value(),
		CreatedAt:    p.CreatedAt.Value(),
	}
}

func SignupFromSchema(p *schema.SignupState) *identityentity.SignupState {
	return &identityentity.SignupState{
		Email:        p.Email,
		PasswordHash: p.PasswordHash,
		TicketHash:   p.TicketHash,
		CodeHash:     p.CodeHash,
		CodeExpireAt: valobj.NewTimeFrom(p.CodeExpireAt),
		CreatedAt:    valobj.NewTimeFrom(p.CreatedAt),
	}
}
