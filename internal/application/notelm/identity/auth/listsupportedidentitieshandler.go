package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

// ListSupportedIdentitiesHandler 返回代码支持的身份类型
type ListSupportedIdentitiesHandler struct{}

func NewListSupportedIdentitiesHandler() *ListSupportedIdentitiesHandler {
	return &ListSupportedIdentitiesHandler{}
}

type SupportedIdentity struct {
	Provider entity.ProviderType
}

type ListSupportedIdentitiesResult struct {
	Identities []*SupportedIdentity
}

func (h *ListSupportedIdentitiesHandler) Handle(_ context.Context) (*ListSupportedIdentitiesResult, error) {
	supported := entity.SupportedProviderTypes()
	items := make([]*SupportedIdentity, 0, len(supported))
	for _, provider := range supported {
		items = append(items, &SupportedIdentity{Provider: provider})
	}

	return &ListSupportedIdentitiesResult{Identities: items}, nil
}
