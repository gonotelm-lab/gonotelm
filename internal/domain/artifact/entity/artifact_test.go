package entity

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewArtifact(t *testing.T) {
	notebookId := uuid.NewV7()
	userId := "user-1"
	payload := &MindmapPayload{NotebookId: notebookId, SourceIds: []valobj.Id{uuid.NewV7()}}

	got := NewArtifact(notebookId, userId, KindMindmap, payload)

	assert.NotEqual(t, valobj.Id{}, got.Id)
	assert.Equal(t, notebookId, got.NotebookId)
	assert.Equal(t, userId, got.UserId)
	assert.Equal(t, KindMindmap, got.Kind)
	assert.Equal(t, StatusPending, got.Status)
	assert.Equal(t, payload, got.Payload)
	assert.False(t, got.IsTerminal())
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
}

func TestArtifactMarkFailed(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	assert.Equal(t, StatusFailed, a.Status)
	assert.True(t, a.IsTerminal())
}

func TestArtifactMarkCancelled(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCancelled()
	assert.Equal(t, StatusCancelled, a.Status)
	assert.True(t, a.IsTerminal())
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
}

func TestArtifactIsOwner(t *testing.T) {
	a := newTestArtifact(t)
	assert.True(t, a.IsOwner("user-1"))
	assert.False(t, a.IsOwner("user-2"))
}

func newTestArtifact(t *testing.T) *Artifact {
	t.Helper()
	return NewArtifact(uuid.NewV7(), "user-1", KindMindmap, &MindmapPayload{})
}
