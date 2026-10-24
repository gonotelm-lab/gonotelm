package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
)

type NotebookAIDescriptionRepositoryImpl struct {
	cache cache.NotebookAIDescriptionCache
}

func NewNotebookAIDescriptionRepository(
	c cache.NotebookAIDescriptionCache,
) notebookrepo.AIDescriptionRepository {
	return &NotebookAIDescriptionRepositoryImpl{cache: c}
}

var _ notebookrepo.AIDescriptionRepository = &NotebookAIDescriptionRepositoryImpl{}

func (r *NotebookAIDescriptionRepositoryImpl) Get(
	ctx context.Context,
	notebookId valobj.Id,
	fingerprint string,
) (string, error) {
	return r.cache.Get(ctx, notebookId.String(), fingerprint)
}

func (r *NotebookAIDescriptionRepositoryImpl) Save(
	ctx context.Context,
	notebookId valobj.Id,
	fingerprint, desc string,
) error {
	return r.cache.Set(ctx, notebookId.String(), fingerprint, desc, cache.NotebookAIDescriptionTTL)
}
