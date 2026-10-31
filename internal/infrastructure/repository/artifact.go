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

func (r *ArtifactRepositoryImpl) Create(ctx context.Context, a *artifactentity.Artifact) error {
	return r.store.Create(ctx, mapper.ArtifactToSchema(a))
}

func (r *ArtifactRepositoryImpl) UpdateExisting(
	ctx context.Context,
	a *artifactentity.Artifact,
) (int64, error) {
	return r.store.UpdateExisting(ctx, mapper.ArtifactToSchema(a))
}

func (r *ArtifactRepositoryImpl) FindById(ctx context.Context, id valobj.Id) (*artifactentity.Artifact, error) {
	sch, err := r.store.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, artifacterrors.ErrArtifactNotFound
		}
		return nil, err
	}

	return mapper.ArtifactFromSchema(sch)
}

func (r *ArtifactRepositoryImpl) PageListByNotebookId(
	ctx context.Context, notebookId valobj.Id, spec *artifactrepo.PageListSpec,
) ([]*artifactentity.Artifact, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	rows, err := r.store.PageListByNotebookId(ctx, notebookId, spec.Limit, spec.Offset)
	if err != nil {
		return nil, err
	}
	out := make([]*artifactentity.Artifact, 0, len(rows))
	for _, row := range rows {
		a, err := mapper.ArtifactFromSchema(row)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}

	return out, nil
}

func (r *ArtifactRepositoryImpl) PageListByStatus(
	ctx context.Context, spec *artifactrepo.PageListByStatusSpec,
) ([]*artifactentity.Artifact, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	strs := make([]string, 0, len(spec.Statuses))
	for _, s := range spec.Statuses {
		strs = append(strs, s.String())
	}
	rows, err := r.store.PageListByStatus(ctx, strs, spec.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]*artifactentity.Artifact, 0, len(rows))
	for _, row := range rows {
		a, err := mapper.ArtifactFromSchema(row)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}

	return out, nil
}

func (r *ArtifactRepositoryImpl) DeleteById(ctx context.Context, id valobj.Id) error {
	return r.store.DeleteById(ctx, id)
}

func (r *ArtifactRepositoryImpl) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return r.store.DeleteByNotebookId(ctx, notebookId)
}
