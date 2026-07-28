package artifact

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type updateArtifactRepoStub struct {
	a     *entity.Artifact
	saved *entity.Artifact
}

func (s *updateArtifactRepoStub) Save(_ context.Context, artifact *entity.Artifact) error {
	s.saved = artifact
	return nil
}

func (s *updateArtifactRepoStub) FindById(_ context.Context, _ valobj.Id) (*entity.Artifact, error) {
	return s.a, nil
}

func (s *updateArtifactRepoStub) ListByNotebookId(context.Context, valobj.Id, *artifactrepo.ListSpec) ([]*entity.Artifact, error) {
	return nil, nil
}

func (s *updateArtifactRepoStub) ListByStatus(context.Context, *artifactrepo.ListByStatusSpec) ([]*entity.Artifact, error) {
	return nil, nil
}

func (s *updateArtifactRepoStub) DeleteById(context.Context, valobj.Id) error { return nil }

func (s *updateArtifactRepoStub) DeleteByNotebookId(context.Context, valobj.Id) error { return nil }

func newCompletedArtifact(t *testing.T, userId string) *entity.Artifact {
	t.Helper()
	a, err := entity.NewArtifact(uuid.NewV7(), userId, entity.KindMindmap, &entity.MindmapPayload{NotebookId: uuid.NewV7()})
	require.NoError(t, err)
	a.MarkCompleted([]byte("r"), entity.ResultKindInline, "old-title")
	return a
}

func TestUpdateArtifactHandler_Title(t *testing.T) {
	a := newCompletedArtifact(t, "user-1")
	repo := &updateArtifactRepoStub{a: a}
	h := NewUpdateArtifactHandler(repo)
	ctx := pkgcontext.WithUserId(context.Background(), "user-1")

	err := h.Handle(ctx, &UpdateCommand{
		ArtifactId: a.Id,
		Target:     UpdateTargetTitle,
		Title:      "  new  ",
	})
	require.NoError(t, err)
	require.NotNil(t, repo.saved)
	assert.Equal(t, "new", repo.saved.Title)
}

func TestUpdateArtifactHandler_UnsupportedTarget(t *testing.T) {
	a := newCompletedArtifact(t, "user-1")
	repo := &updateArtifactRepoStub{a: a}
	h := NewUpdateArtifactHandler(repo)
	ctx := pkgcontext.WithUserId(context.Background(), "user-1")

	err := h.Handle(ctx, &UpdateCommand{
		ArtifactId: a.Id,
		Target:     UpdateTarget("bogus"),
		Title:      "x",
	})
	assert.ErrorIs(t, err, errors.ErrParams)
}

func TestUpdateArtifactHandler_NotOwner(t *testing.T) {
	a := newCompletedArtifact(t, "user-1")
	repo := &updateArtifactRepoStub{a: a}
	h := NewUpdateArtifactHandler(repo)
	ctx := pkgcontext.WithUserId(context.Background(), "other-user")

	err := h.Handle(ctx, &UpdateCommand{
		ArtifactId: a.Id,
		Target:     UpdateTargetTitle,
		Title:      "x",
	})
	assert.ErrorIs(t, err, artifacterrors.ErrArtifactNotOwnedByUser)
}
