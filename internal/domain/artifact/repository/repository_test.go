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
func (f *fakeRepo) ListByNotebookId(ctx context.Context, n valobj.Id, spec *ListSpec) ([]*entity.Artifact, error) {
	return nil, nil
}
func (f *fakeRepo) ListByStatus(ctx context.Context, spec *ListByStatusSpec) ([]*entity.Artifact, error) {
	return nil, nil
}
func (f *fakeRepo) DeleteById(ctx context.Context, id valobj.Id) error       { return nil }
func (f *fakeRepo) DeleteByNotebookId(ctx context.Context, n valobj.Id) error { return nil }

func TestRepositoryInterfaceSatisfied(t *testing.T) {
	_ = &fakeRepo{}
}
