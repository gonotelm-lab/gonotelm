package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

// ListIdentitiesHandler 列出当前账号已绑定的三方身份
type ListIdentitiesHandler struct {
	userService *identityservice.UserService
}

func NewListIdentitiesHandler(userService *identityservice.UserService) *ListIdentitiesHandler {
	return &ListIdentitiesHandler{userService: userService}
}

type IdentityItem struct {
	Id                    valobj.Id
	Provider              entity.ProviderType
	ProviderEmail         string
	ProviderEmailVerified bool
	BoundAt               valobj.Time
	LastLoginAt           valobj.Time
}

type ListIdentitiesResult struct {
	Identities []*IdentityItem
}

func (h *ListIdentitiesHandler) Handle(ctx context.Context) (*ListIdentitiesResult, error) {
	agg, err := h.userService.GetAccount(ctx, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, err
	}

	identities := agg.Identities()
	items := make([]*IdentityItem, 0, len(identities))
	for _, identity := range identities {
		items = append(items, &IdentityItem{
			Id:                    identity.Id,
			Provider:              identity.Provider,
			ProviderEmail:         identity.ProviderEmail,
			ProviderEmailVerified: identity.ProviderEmailVerified,
			BoundAt:               identity.CreatedAt,
			LastLoginAt:           identity.LastLoginAt,
		})
	}

	return &ListIdentitiesResult{Identities: items}, nil
}
