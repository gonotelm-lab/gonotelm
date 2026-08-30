package repository

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sandboxerrors "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/errors"
	sandboxrepo "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SandboxRepositoryImpl struct {
	sandboxCache cache.SandboxCache
}

func NewSandboxRepository(sandboxCache cache.SandboxCache) sandboxrepo.Repository {
	return &SandboxRepositoryImpl{
		sandboxCache: sandboxCache,
	}
}

var _ sandboxrepo.Repository = &SandboxRepositoryImpl{}

func (r *SandboxRepositoryImpl) GetSandbox(ctx context.Context, key entity.SandboxKey) (entity.SandboxDescription, error) {
	desc, err := r.sandboxCache.Get(ctx, key.UserId.String(), key.NotebookId.String())
	if err != nil {
		return entity.SandboxDescription{}, err
	}
	if desc == nil {
		return entity.SandboxDescription{}, sandboxerrors.ErrSandboxNotFound
	}

	notebookId, err := valobj.NewIdFromString(desc.Key.NotebookId)
	if err != nil {
		return entity.SandboxDescription{}, pkgerr.Wrapf(pkgerr.ErrSerde, "invalid sandbox desc notebook id: %s", err.Error())
	}

	userId, err := valobj.NewUidFromString(desc.Key.UserId)
	if err != nil {
		return entity.SandboxDescription{}, pkgerr.Wrapf(pkgerr.ErrSerde, "invalid sandbox desc user id: %s", err.Error())
	}

	return entity.SandboxDescription{
		Id: desc.Id,
		Key: entity.SandboxKey{
			UserId:     userId,
			NotebookId: notebookId,
		},
		Runtime: desc.Runtime,
	}, nil
}

func (r *SandboxRepositoryImpl) DeleteSandbox(ctx context.Context, key entity.SandboxKey) error {
	return r.sandboxCache.Delete(ctx, key.UserId.String(), key.NotebookId.String())
}

func (r *SandboxRepositoryImpl) SetSandbox(ctx context.Context, key entity.SandboxKey, desc entity.SandboxDescription, ttl time.Duration) error {
	cacheDesc := &schema.SandboxDescription{
		Id: desc.Id,
		Key: schema.SandboxKey{
			UserId:     desc.Key.UserId.String(),
			NotebookId: desc.Key.NotebookId.String(),
		},
		Runtime: desc.Runtime,
	}

	return r.sandboxCache.Set(ctx, key.UserId.String(), key.NotebookId.String(), cacheDesc, ttl)
}
