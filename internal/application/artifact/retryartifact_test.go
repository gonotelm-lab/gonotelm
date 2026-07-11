package artifact

import (
	"context"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry_Execute_SubmitsWorkerInput(t *testing.T) {
	notebookId := uuid.NewV7()
	sourceIds := []valobj.Id{uuid.NewV7()}
	a, err := artifactentity.NewArtifact(notebookId, "u1", artifactentity.KindMindmap, &artifactentity.MindmapPayload{
		NotebookId: notebookId,
		SourceIds:  sourceIds,
	})
	require.NoError(t, err)
	a.MarkFailed()

	repo := &multiStubRepo{findByIdResult: a}
	flowc := &capturingFlowClient{submitID: "flow-retry-1"}
	h := NewRetryArtifactHandler(repo, flowc, nil, &stubEventBus{})

	ctx := pkgcontext.WithUserId(context.Background(), "u1")
	err = h.Handle(ctx, a.Id)

	require.NoError(t, err)
	assert.Equal(t, "artifact.mindmap", flowc.submittedType)

	var workerInput generate.WorkerInput
	err = sonic.Unmarshal(flowc.submittedPayload, &workerInput)
	require.NoError(t, err)
	assert.Equal(t, a.Id.String(), workerInput.ArtifactId)
	assert.Equal(t, notebookId.String(), workerInput.NotebookId)
	assert.Equal(t, "u1", workerInput.UserId)
	assert.Equal(t, []string{sourceIds[0].String()}, workerInput.SourceIds)
	assert.Equal(t, string(artifactentity.KindMindmap), workerInput.Kind)
	assert.NotNil(t, workerInput.Payload)
}
