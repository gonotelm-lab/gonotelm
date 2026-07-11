package repository

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ArtifactRepositoryImpl struct{ store database.ArtifactStore }

func NewArtifactRepository(store database.ArtifactStore) artifactrepo.Repository {
	return &ArtifactRepositoryImpl{store: store}
}

var _ artifactrepo.Repository = &ArtifactRepositoryImpl{}

func (r *ArtifactRepositoryImpl) Save(ctx context.Context, a *artifactentity.Artifact) error {
	return r.store.Create(ctx, mapper.ArtifactToSchema(a))
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

func (r *ArtifactRepositoryImpl) ListByNotebookId(ctx context.Context, notebookId valobj.Id, limit, offset int) ([]*artifactentity.Artifact, error) {
	rows, err := r.store.ListByNotebookId(ctx, notebookId, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*artifactentity.Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.ArtifactFromSchema(row))
	}
	return out, nil
}

func (r *ArtifactRepositoryImpl) ListByStatus(ctx context.Context, statuses []artifactentity.Status, limit int) ([]*artifactentity.Artifact, error) {
	strs := make([]string, 0, len(statuses))
	for _, s := range statuses {
		strs = append(strs, s.String())
	}
	rows, err := r.store.ListByStatus(ctx, strs, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*artifactentity.Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.ArtifactFromSchema(row))
	}
	return out, nil
}

func (r *ArtifactRepositoryImpl) UpdateStatus(
	ctx context.Context,
	id valobj.Id,
	status artifactentity.Status,
	result []byte,
	resultKind artifactentity.ResultKind,
	title string,
) error {
	_, err := r.store.UpdateStatus(ctx, id, status.String(), "", &schema.ArtifactUpdateStatusParams{
		NewStatus: status.String(), Title: title, Result: result, ResultKind: resultKind.String(), UpdatedAt: time.Now(),
	})
	return err
}

func (r *ArtifactRepositoryImpl) UpdateFlowTaskId(ctx context.Context, id valobj.Id, flowTaskId string, oldStatuses []artifactentity.Status) error {
	strs := make([]string, 0, len(oldStatuses))
	for _, s := range oldStatuses {
		strs = append(strs, s.String())
	}
	return r.store.UpdateFlowTaskId(ctx, id, flowTaskId, strs)
}

func (r *ArtifactRepositoryImpl) DeleteById(ctx context.Context, id valobj.Id) error {
	return r.store.DeleteById(ctx, id)
}

func (r *ArtifactRepositoryImpl) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return r.store.DeleteByNotebookId(ctx, notebookId)
}