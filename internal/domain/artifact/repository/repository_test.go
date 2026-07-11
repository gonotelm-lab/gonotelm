package repository

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type fakeRepo struct{}

var _ Repository = &fakeRepo{}

func (f *fakeRepo) Save(ctx context.Context, a *entity.Artifact) error { return nil }
func (f *fakeRepo) FindById(ctx context.Context, id valobj.Id) (*entity.Artifact, error) {
	return nil, nil
}
func (f *fakeRepo) ListByNotebookId(ctx context.Context, notebookId valobj.Id, limit, offset int) ([]*entity.Artifact, error) {
	return nil, nil
}
func (f *fakeRepo) ListByStatus(ctx context.Context, statuses []entity.Status, limit int) ([]*entity.Artifact, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateStatus(ctx context.Context, id valobj.Id, status entity.Status, result []byte, resultKind entity.ResultKind, title string) error {
	return nil
}
func (f *fakeRepo) UpdateFlowTaskId(ctx context.Context, id valobj.Id, flowTaskId string, oldStatuses []entity.Status) error {
	return nil
}
func (f *fakeRepo) DeleteById(ctx context.Context, id valobj.Id) error                 { return nil }
func (f *fakeRepo) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error { return nil }

func TestRepositoryInterfaceSatisfied(t *testing.T) {
	_ = &fakeRepo{}
}
