package auth

import (
	"sort"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
)

type ProvidersHandler struct {
	loginRepo repository.LoginInfoRepository
}

func NewProvidersHandler(loginRepo repository.LoginInfoRepository) *ProvidersHandler {
	return &ProvidersHandler{loginRepo: loginRepo}
}

func (h *ProvidersHandler) Handle() []string {
	providers := h.loginRepo.AvailableProviders()

	types := make([]string, 0, len(providers))
	for _, provider := range providers {
		types = append(types, provider.Type().String())
	}
	sort.Strings(types)

	return types
}
