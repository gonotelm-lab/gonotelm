package auth

import (
	"sort"

	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
)

// ProvidersHandler 只列当前部署配好凭据、能真正完成登录的 provider。
// 它只依赖注册表，不依赖任何状态存储。
type ProvidersHandler struct {
	registry identrepo.ProviderRegistry
}

func NewProvidersHandler(registry identrepo.ProviderRegistry) *ProvidersHandler {
	return &ProvidersHandler{registry: registry}
}

func (h *ProvidersHandler) Handle() []string {
	providers := h.registry.AvailableProviders()

	types := make([]string, 0, len(providers))
	for _, provider := range providers {
		types = append(types, provider.Type().String())
	}
	sort.Strings(types)

	return types
}
