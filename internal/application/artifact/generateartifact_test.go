package artifact

import (
	"context"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubArtifactRepo struct {
	saved []*artifactentity.Artifact
	err   error
}

func (s *stubArtifactRepo) Save(ctx context.Context, a *artifactentity.Artifact) error {
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, a)
	return nil
}
func (s *stubArtifactRepo) FindById(ctx context.Context, id valobj.Id) (*artifactentity.Artifact, error) {
	return nil, artifacterrors.ErrArtifactNotFound
}
func (s *stubArtifactRepo) ListByNotebookId(ctx context.Context, n valobj.Id, spec *artifactrepo.ListSpec) ([]*artifactentity.Artifact, error) {
	return nil, nil
}
func (s *stubArtifactRepo) ListByStatus(ctx context.Context, spec *artifactrepo.ListByStatusSpec) ([]*artifactentity.Artifact, error) {
	return nil, nil
}
func (s *stubArtifactRepo) DeleteById(ctx context.Context, id valobj.Id) error { return nil }
func (s *stubArtifactRepo) DeleteByNotebookId(ctx context.Context, n valobj.Id) error {
	return nil
}

var _ artifactrepo.Repository = &stubArtifactRepo{}

type stubFlowClient struct {
	submitID  string
	submitErr error
	canceled  []string
	getInfo   *flow.TaskInfo
	getErr    error
}

func (s *stubFlowClient) Submit(ctx context.Context, t string, p []byte) (string, error) {
	return s.submitID, s.submitErr
}
func (s *stubFlowClient) Get(ctx context.Context, id string) (*flow.TaskInfo, error) {
	if s.getInfo != nil {
		return s.getInfo, s.getErr
	}
	return nil, nil
}
func (s *stubFlowClient) Cancel(ctx context.Context, id string) error {
	s.canceled = append(s.canceled, id)
	return nil
}
func (s *stubFlowClient) Close() error { return nil }

var _ flow.TaskClient = &stubFlowClient{}

type capturingFlowClient struct {
	submitID         string
	submitErr        error
	submittedType    string
	submittedPayload []byte
}

func (s *capturingFlowClient) Submit(ctx context.Context, t string, p []byte) (string, error) {
	s.submittedType = t
	s.submittedPayload = p
	return s.submitID, s.submitErr
}
func (s *capturingFlowClient) Get(ctx context.Context, id string) (*flow.TaskInfo, error) {
	return nil, nil
}
func (s *capturingFlowClient) Cancel(ctx context.Context, id string) error { return nil }
func (s *capturingFlowClient) Close() error                                { return nil }

var _ flow.TaskClient = &capturingFlowClient{}

type stubNotebookRepo struct {
	ownerId string
	err     error
}

func (s *stubNotebookRepo) Save(ctx context.Context, nb *notebookentity.Notebook) error { return nil }
func (s *stubNotebookRepo) FindById(ctx context.Context, id valobj.Id) (*notebookentity.Notebook, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &notebookentity.Notebook{OwnerId: s.ownerId}, nil
}
func (s *stubNotebookRepo) ListByOwner(ctx context.Context, ownerId string, spec *notebookrepo.ListSpec) ([]*notebookentity.Notebook, error) {
	return nil, nil
}

var _ notebookrepo.Repository = &stubNotebookRepo{}

func TestGenerate_Execute_HappyPath(t *testing.T) {
	repo := &stubArtifactRepo{}
	flowc := &stubFlowClient{submitID: "flow-1"}
	notebookRepo := &stubNotebookRepo{ownerId: "u1"}
	h := NewGenerateArtifactHandler(repo, flowc, notebookRepo, nil, &stubEventBus{})

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	resp, err := h.Handle(ctx, &GenerateRequest{
		NotebookId: uuid.NewV7(),
		Kind:       artifactentity.KindMindmap,
		SourceIds:  []valobj.Id{uuid.NewV7()},
	})

	require.NoError(t, err)
	assert.NotEqual(t, valobj.Id(uuid.EmptyUUID()), resp.ArtifactId)
	assert.Equal(t, "flow-1", repo.saved[0].FlowTaskId)
}

func TestGenerate_Execute_NotebookOwnedByOther(t *testing.T) {
	repo := &stubArtifactRepo{}
	flowc := &stubFlowClient{}
	notebookRepo := &stubNotebookRepo{ownerId: "other-user"}
	h := NewGenerateArtifactHandler(repo, flowc, notebookRepo, nil, &stubEventBus{})

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	_, err := h.Handle(ctx, &GenerateRequest{
		NotebookId: uuid.NewV7(),
		Kind:       artifactentity.KindMindmap,
		SourceIds:  nil,
	})

	require.Error(t, err)
}

func TestGenerate_Execute_SubmitsWorkerInput(t *testing.T) {
	repo := &stubArtifactRepo{}
	flowc := &capturingFlowClient{submitID: "flow-1"}
	notebookRepo := &stubNotebookRepo{ownerId: "u1"}
	h := NewGenerateArtifactHandler(repo, flowc, notebookRepo, nil, &stubEventBus{})

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	notebookId := uuid.NewV7()
	sourceIds := []valobj.Id{uuid.NewV7()}
	resp, err := h.Handle(ctx, &GenerateRequest{
		NotebookId: notebookId,
		Kind:       artifactentity.KindMindmap,
		SourceIds:  sourceIds,
	})

	require.NoError(t, err)
	assert.NotEqual(t, valobj.Id(uuid.EmptyUUID()), resp.ArtifactId)
	assert.Equal(t, "artifact.mindmap", flowc.submittedType)

	var workerInput generate.WorkerInput
	err = sonic.Unmarshal(flowc.submittedPayload, &workerInput)
	require.NoError(t, err)
	assert.Equal(t, resp.ArtifactId.String(), workerInput.ArtifactId)
	assert.Equal(t, notebookId.String(), workerInput.NotebookId)
	assert.Equal(t, "u1", workerInput.UserId)
	assert.Equal(t, []string{sourceIds[0].String()}, workerInput.SourceIds)
	assert.Equal(t, string(artifactentity.KindMindmap), workerInput.Kind)
	assert.NotNil(t, workerInput.Payload)
}
