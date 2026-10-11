package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

type ProviderRegistry interface {
	AvailableProviders() []entity.Provider
	GetProvider(ctx context.Context, providerType entity.ProviderType) (entity.Provider, error)
}
