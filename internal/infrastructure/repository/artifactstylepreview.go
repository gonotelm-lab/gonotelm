package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type StylePreviewRepositoryImpl struct {
	store database.ArtifactStylePreviewStore
}

func NewStylePreviewRepository(store database.ArtifactStylePreviewStore) artifactrepo.StylePreviewRepository {
	return &StylePreviewRepositoryImpl{store: store}
}

var _ artifactrepo.StylePreviewRepository = &StylePreviewRepositoryImpl{}

func (r *StylePreviewRepositoryImpl) Save(ctx context.Context, p *entity.StylePreview) error {
	return r.store.Upsert(ctx, mapper.StylePreviewToSchema(p))
}

func (r *StylePreviewRepositoryImpl) FindByIdentifier(
	ctx context.Context, identifier entity.StylePreviewIdentifier,
) (*entity.StylePreview, error) {
	sch, err := r.store.GetByIdentifier(ctx, identifier.String())
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, artifacterrors.ErrStylePreviewNotFound
		}
		return nil, err
	}

	return mapper.StylePreviewFromSchema(sch)
}

func (r *StylePreviewRepositoryImpl) ListByIdentifiers(
	ctx context.Context, identifiers []entity.StylePreviewIdentifier,
) ([]*entity.StylePreview, error) {
	ids := make([]string, 0, len(identifiers))
	for _, id := range identifiers {
		ids = append(ids, id.String())
	}

	rows, err := r.store.ListByIdentifiers(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*entity.StylePreview, 0, len(rows))
	for _, row := range rows {
		p, err := mapper.StylePreviewFromSchema(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return out, nil
}

func (r *StylePreviewRepositoryImpl) DeleteByIdentifier(ctx context.Context, identifier entity.StylePreviewIdentifier) error {
	return r.store.DeleteByIdentifier(ctx, identifier.String())
}
