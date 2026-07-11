package artifact

import (
	"context"
	"testing"

	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type multiStubRepo struct {
	findByIdResult *artifactentity.Artifact
	findByIdErr    error
	updatedStatus  *artifactentity.Status
	updatedFTID    string
	updatedFTOld   []artifactentity.Status
	listResult     []*artifactentity.Artifact
	listErr        error
	deletedId      valobj.Id
	deleteErr      error
}

func (s *multiStubRepo) Save(ctx context.Context, a *artifactentity.Artifact) error { return nil }
func (s *multiStubRepo) FindById(ctx context.Context, id valobj.Id) (*artifactentity.Artifact, error) {
	return s.findByIdResult, s.findByIdErr
}
func (s *multiStubRepo) ListByNotebookId(ctx context.Context, n valobj.Id, l, o int) ([]*artifactentity.Artifact, error) {
	return s.listResult, s.listErr
}
func (s *multiStubRepo) ListByStatus(ctx context.Context, sts []artifactentity.Status, l int) ([]*artifactentity.Artifact, error) {
	return nil, nil
}
func (s *multiStubRepo) UpdateStatus(ctx context.Context, id valobj.Id, st artifactentity.Status, r []byte, rk artifactentity.ResultKind, t string) error {
	s.updatedStatus = &st
	return nil
}
func (s *multiStubRepo) UpdateFlowTaskId(ctx context.Context, id valobj.Id, flowTaskId string, oldStatuses []artifactentity.Status) error {
	s.updatedFTID = flowTaskId
	s.updatedFTOld = oldStatuses
	return nil
}
func (s *multiStubRepo) DeleteById(ctx context.Context, id valobj.Id) error {
	s.deletedId = id
	return s.deleteErr
}
func (s *multiStubRepo) DeleteByNotebookId(ctx context.Context, n valobj.Id) error { return nil }

var _ artifactrepo.Repository = &multiStubRepo{}

type stubStorage struct {
	deletedKey string
}

func (s *stubStorage) DeleteObject(ctx context.Context, key string) error {
	s.deletedKey = key
	return nil
}
func (s *stubStorage) PresignGet(ctx context.Context, key string) (string, error) { return "", nil }

var _ StorageGateway = &stubStorage{}

func makeArtifact(status artifactentity.Status, flowTaskId string, userId string) *artifactentity.Artifact {
	a := artifactentity.NewArtifact(uuid.NewV7(), userId, artifactentity.KindMindmap, &artifactentity.MindmapPayload{NotebookId: uuid.NewV7()})
	a.Status = status
	a.FlowTaskId = flowTaskId
	if status == artifactentity.StatusCompleted {
		a.Result = []byte(`{"StoreKey":"key-1"}`)
		a.ResultKind = artifactentity.ResultKindStorage
		a.Title = "done title"
	}
	return a
}

func TestStatus_TerminalArtifact(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusCompleted, "ft-1", "u1")
	repo := &multiStubRepo{findByIdResult: artifact}
	flowc := &stubFlowClient{}
	h := NewGetArtifactStatusHandler(repo, flowc)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	resp, err := h.Handle(ctx, &StatusRequest{ArtifactId: artifact.Id})

	require.NoError(t, err)
	assert.Equal(t, artifactentity.StatusCompleted, resp.Status)
	assert.Equal(t, "done title", resp.Title)
	assert.NotNil(t, resp.Result)
	assert.Equal(t, artifactentity.ResultKindStorage, resp.ResultKind)
}

func TestStatus_ActiveArtifact_FlowGet(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusRunning, "ft-2", "u1")
	repo := &multiStubRepo{findByIdResult: artifact}
	flowc := &stubFlowClient{getInfo: &flow.TaskInfo{State: flowschema.TaskState_RUNNING}}
	h := NewGetArtifactStatusHandler(repo, flowc)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	resp, err := h.Handle(ctx, &StatusRequest{ArtifactId: artifact.Id})

	require.NoError(t, err)
	assert.Equal(t, artifactentity.StatusRunning, resp.Status)
}

func TestStatus_PermissionDenied(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusPending, "", "u2")
	repo := &multiStubRepo{findByIdResult: artifact}
	h := NewGetArtifactStatusHandler(repo, &stubFlowClient{})

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	_, err := h.Handle(ctx, &StatusRequest{ArtifactId: artifact.Id})

	require.Error(t, err)
	assert.ErrorIs(t, err, artifacterrors.ErrArtifactNotOwnedByUser)
}

func TestList_HappyPath(t *testing.T) {
	repo := &multiStubRepo{
		listResult: []*artifactentity.Artifact{
			makeArtifact(artifactentity.StatusCompleted, "ft-a", "u1"),
			makeArtifact(artifactentity.StatusRunning, "ft-b", "u1"),
		},
	}
	nbRepo := &stubNotebookRepo{ownerId: "u1"}
	h := NewListArtifactsHandler(repo, nbRepo)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	resp, err := h.Handle(ctx, &ListRequest{NotebookId: uuid.NewV7(), Limit: 5, Offset: 0})

	require.NoError(t, err)
	assert.Len(t, resp.Artifacts, 2)
	assert.False(t, resp.HasMore)
}

func TestList_HasMore(t *testing.T) {
	artifacts := make([]*artifactentity.Artifact, 4)
	for i := range artifacts {
		artifacts[i] = makeArtifact(artifactentity.StatusPending, "", "u1")
	}
	repo := &multiStubRepo{listResult: artifacts}
	nbRepo := &stubNotebookRepo{ownerId: "u1"}
	h := NewListArtifactsHandler(repo, nbRepo)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	resp, err := h.Handle(ctx, &ListRequest{NotebookId: uuid.NewV7(), Limit: 3, Offset: 0})

	require.NoError(t, err)
	assert.Len(t, resp.Artifacts, 3)
	assert.True(t, resp.HasMore)
}

func TestCancel_HappyPath(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusRunning, "ft-3", "u1")
	repo := &multiStubRepo{findByIdResult: artifact}
	flowc := &stubFlowClient{}
	h := NewCancelArtifactHandler(repo, flowc)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	err := h.Handle(ctx, artifact.Id)

	require.NoError(t, err)
	assert.Len(t, flowc.canceled, 1)
	assert.Equal(t, "ft-3", flowc.canceled[0])
	assert.NotNil(t, repo.updatedStatus)
	assert.Equal(t, artifactentity.StatusCancelled, *repo.updatedStatus)
}

func TestCancel_TerminalRejected(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusCompleted, "ft-4", "u1")
	repo := &multiStubRepo{findByIdResult: artifact}
	h := NewCancelArtifactHandler(repo, &stubFlowClient{})

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	err := h.Handle(ctx, artifact.Id)

	require.Error(t, err)
	assert.ErrorIs(t, err, artifacterrors.ErrCannotCancelInState)
}

func TestDelete_HappyPath(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusCompleted, "ft-5", "u1")
	repo := &multiStubRepo{findByIdResult: artifact}
	storage := &stubStorage{}
	h := NewDeleteArtifactHandler(repo, &stubFlowClient{}, storage)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	err := h.Handle(ctx, artifact.Id)

	require.NoError(t, err)
	assert.Equal(t, artifact.Id, repo.deletedId)
	assert.Equal(t, "key-1", storage.deletedKey)
}

func TestDelete_NonTerminalCancelsFlow(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusRunning, "ft-6", "u1")
	repo := &multiStubRepo{findByIdResult: artifact}
	flowc := &stubFlowClient{}
	h := NewDeleteArtifactHandler(repo, flowc, &stubStorage{})

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	err := h.Handle(ctx, artifact.Id)

	require.NoError(t, err)
	assert.Len(t, flowc.canceled, 1)
	assert.Equal(t, "ft-6", flowc.canceled[0])
	assert.Equal(t, artifact.Id, repo.deletedId)
}

func TestRetry_HappyPath(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusFailed, "ft-old", "u1")
	artifact.Payload = &artifactentity.MindmapPayload{NotebookId: artifact.NotebookId}
	repo := &multiStubRepo{findByIdResult: artifact}
	flowc := &stubFlowClient{submitID: "ft-new"}
	h := NewRetryArtifactHandler(repo, flowc, nil)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	err := h.Handle(ctx, artifact.Id)

	require.NoError(t, err)
	assert.Equal(t, "ft-new", repo.updatedFTID)
	assert.Equal(t, []artifactentity.Status{artifactentity.StatusFailed, artifactentity.StatusCancelled}, repo.updatedFTOld)
	assert.NotNil(t, repo.updatedStatus)
	assert.Equal(t, artifactentity.StatusPending, *repo.updatedStatus)
}

func TestRetry_CannotRetryTerminalButNotFailedOrCancelled(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusCompleted, "ft-c", "u1")
	repo := &multiStubRepo{findByIdResult: artifact}
	h := NewRetryArtifactHandler(repo, &stubFlowClient{}, nil)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	err := h.Handle(ctx, artifact.Id)

	require.Error(t, err)
	assert.ErrorIs(t, err, artifacterrors.ErrCannotRetryInState)
}

func TestRetry_CannotRetryPending(t *testing.T) {
	artifact := makeArtifact(artifactentity.StatusPending, "ft-p", "u1")
	repo := &multiStubRepo{findByIdResult: artifact}
	h := NewRetryArtifactHandler(repo, &stubFlowClient{}, nil)

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	err := h.Handle(ctx, artifact.Id)

	require.Error(t, err)
	assert.ErrorIs(t, err, artifacterrors.ErrCannotRetryInState)
}
