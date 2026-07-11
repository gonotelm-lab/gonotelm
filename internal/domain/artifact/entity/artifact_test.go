package entity

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactevent "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/event"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewArtifact(t *testing.T) {
	notebookId := uuid.NewV7()
	userId := "user-1"
	payload := &MindmapPayload{NotebookId: notebookId, SourceIds: []valobj.Id{uuid.NewV7()}}

	got, err := NewArtifact(notebookId, userId, KindMindmap, payload)
	require.NoError(t, err)

	assert.NotEqual(t, valobj.Id{}, got.Id)
	assert.Equal(t, notebookId, got.NotebookId)
	assert.Equal(t, userId, got.UserId)
	assert.Equal(t, KindMindmap, got.Kind)
	assert.Equal(t, StatusPending, got.Status)
	assert.Equal(t, payload, got.Payload)
	assert.False(t, got.IsTerminal())
}

func TestNewArtifact_Validation_Errors(t *testing.T) {
	notebookId := uuid.NewV7()
	validPayload := &MindmapPayload{NotebookId: notebookId}

	tests := []struct {
		name       string
		notebookId valobj.Id
		userId     string
		kind       Kind
		payload    Payload
		errTarget  error
	}{
		{"empty notebook id", valobj.Id{}, "u1", KindMindmap, validPayload, artifacterrors.ErrInvalidNotebookId},
		{"empty user id", notebookId, "", KindMindmap, validPayload, artifacterrors.ErrInvalidUserId},
		{"unsupported kind", notebookId, "u1", Kind("bogus"), validPayload, artifacterrors.ErrInvalidKind},
		{"nil payload", notebookId, "u1", KindMindmap, nil, artifacterrors.ErrInvalidPayload},
		{"payload kind mismatch", notebookId, "u1", KindMindmap, &ReportPayload{NotebookId: notebookId}, artifacterrors.ErrPayloadKindMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewArtifact(tt.notebookId, tt.userId, tt.kind, tt.payload)
			assert.ErrorIs(t, err, tt.errTarget)
		})
	}
}

func TestNewArtifact_SetsUpdateTime(t *testing.T) {
	a, err := NewArtifact(uuid.NewV7(), "u1", KindMindmap, &MindmapPayload{NotebookId: uuid.NewV7()})
	require.NoError(t, err)
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactBindFlowTaskId(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("flow-task-1")
	assert.Equal(t, "flow-task-1", a.FlowTaskId)
}

func TestArtifactMarkCompleted(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCompleted([]byte("result"), ResultKindInline, "title")
	assert.Equal(t, StatusCompleted, a.Status)
	assert.Equal(t, []byte("result"), a.Result)
	assert.Equal(t, ResultKindInline, a.ResultKind)
	assert.Equal(t, "title", a.Title)
	assert.True(t, a.IsTerminal())
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactMarkFailed(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	assert.Equal(t, StatusFailed, a.Status)
	assert.True(t, a.IsTerminal())
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactMarkCancelled(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCancelled()
	assert.Equal(t, StatusCancelled, a.Status)
	assert.True(t, a.IsTerminal())
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactMarkRetrying(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCompleted([]byte("old"), ResultKindInline, "old-title")
	a.MarkRetrying("flow-task-2")
	assert.Equal(t, StatusPending, a.Status)
	assert.Equal(t, "flow-task-2", a.FlowTaskId)
	assert.Empty(t, a.Title)
	assert.Empty(t, a.Result)
	assert.Empty(t, a.ResultKind)
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactCancel_HappyPath(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("ft-1")
	require.NoError(t, a.Cancel())
	assert.Equal(t, StatusCancelled, a.Status)
}

func TestArtifactCancel_TerminalRejected(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("ft-1")
	a.MarkCompleted([]byte("r"), ResultKindInline, "t")
	err := a.Cancel()
	assert.ErrorIs(t, err, artifacterrors.ErrCannotCancelInState)
}

func TestArtifactCancel_NoFlowTaskId(t *testing.T) {
	a := newTestArtifact(t)
	err := a.Cancel()
	assert.ErrorIs(t, err, artifacterrors.ErrInvalidFlowTaskId)
}

func TestArtifactRetry_HappyPath(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	require.NoError(t, a.Retry("ft-2"))
	assert.Equal(t, StatusPending, a.Status)
	assert.Equal(t, "ft-2", a.FlowTaskId)
}

func TestArtifactRetry_NotFailedOrCancelled(t *testing.T) {
	a := newTestArtifact(t)
	err := a.Retry("ft-2")
	assert.ErrorIs(t, err, artifacterrors.ErrCannotRetryInState)
}

func TestArtifactIsOwner(t *testing.T) {
	a := newTestArtifact(t)
	assert.True(t, a.IsOwner("user-1"))
	assert.False(t, a.IsOwner("user-2"))
}

func TestArtifactMarkCompleted_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCompleted([]byte("r"), ResultKindInline, "title")
	events := a.PullEvents()
	require.Len(t, events, 1)
	assert.Equal(t, artifactevent.TopicArtifactEvent, events[0].Topic())
}

func TestArtifactMarkFailed_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	events := a.PullEvents()
	require.Len(t, events, 1)
}

func TestArtifactMarkCancelled_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCancelled()
	events := a.PullEvents()
	require.Len(t, events, 1)
}

func TestArtifactCancel_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("ft-1")
	require.NoError(t, a.Cancel())
	events := a.PullEvents()
	require.Len(t, events, 1)
}

func TestArtifactRetry_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	a.PullEvents()
	require.NoError(t, a.Retry("ft-2"))
	events := a.PullEvents()
	require.Len(t, events, 1)
}

func newTestArtifact(t *testing.T) *Artifact {
	t.Helper()
	a, err := NewArtifact(uuid.NewV7(), "user-1", KindMindmap, &MindmapPayload{NotebookId: uuid.NewV7()})
	require.NoError(t, err)
	return a
}
