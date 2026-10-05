package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

type LoginInfoRepository interface {
	AvailableProviders() []entity.Provider
	GetProvider(ctx context.Context, providerType entity.ProviderType) (entity.Provider, error)
	// save trasient login info
	SaveTransientLoginInfo(ctx context.Context, loginInfo *entity.TransientProviderLoginInfo) error
	GetTransientLoginInfo(ctx context.Context, state string) (*entity.TransientProviderLoginInfo, error)
	DeleteTransientLoginInfo(ctx context.Context, state string) error
}
