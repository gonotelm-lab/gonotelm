package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ArtifactRepositoryImpl struct{ store database.ArtifactStore }

func NewArtifactRepository(store database.ArtifactStore) artifactrepo.Repository {
	return &ArtifactRepositoryImpl{store: store}
}

var _ artifactrepo.Repository = &ArtifactRepositoryImpl{}

func (r *ArtifactRepositoryImpl) Save(ctx context.Context, a *artifactentity.Artifact) error {
	if a.IsDeleted() {
		return r.store.DeleteById(ctx, a.Id)
	}
	return r.store.Upsert(ctx, mapper.ArtifactToSchema(a))
}

func (r *ArtifactRepositoryImpl) FindById(ctx context.Context, id valobj.Id) (*artifactentity.Artifact, error) {
	sch, err := r.store.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, artifacterrors.ErrArtifactNotFound
		}
		return nil, err
	}
	return mapper.ArtifactFromSchema(sch), nil
}

func (r *ArtifactRepositoryImpl) ListByNotebookId(
	ctx context.Context, notebookId valobj.Id, spec *artifactrepo.ListSpec,
) ([]*artifactentity.Artifact, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	rows, err := r.store.ListByNotebookId(ctx, notebookId, spec.Limit, spec.Offset)
	if err != nil {
		return nil, err
	}
	out := make([]*artifactentity.Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.ArtifactFromSchema(row))
	}
	return out, nil
}

func (r *ArtifactRepositoryImpl) ListByStatus(
	ctx context.Context, spec *artifactrepo.ListByStatusSpec,
) ([]*artifactentity.Artifact, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	strs := make([]string, 0, len(spec.Statuses))
	for _, s := range spec.Statuses {
		strs = append(strs, s.String())
	}
	rows, err := r.store.ListByStatus(ctx, strs, spec.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]*artifactentity.Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.ArtifactFromSchema(row))
	}
	return out, nil
}

func (r *ArtifactRepositoryImpl) DeleteById(ctx context.Context, id valobj.Id) error {
	return r.store.DeleteById(ctx, id)
}

func (r *ArtifactRepositoryImpl) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return r.store.DeleteByNotebookId(ctx, notebookId)
}
