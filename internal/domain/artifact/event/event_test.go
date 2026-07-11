package event

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCompletedEvent(t *testing.T) {
	id, nbId := uuid.NewV7(), uuid.NewV7()
	evt := NewCompletedEvent(id, nbId)
	assert.Equal(t, TopicArtifactEvent, evt.Topic())
	assert.Equal(t, id.String(), evt.Key())
	assert.Equal(t, event.CategoryInner, evt.Category())
	assert.Equal(t, ActionCompleted, evt.Action())
	assert.Equal(t, id, evt.ArtifactId())
	assert.Equal(t, nbId, evt.NotebookId())
}

func TestFailedEvent(t *testing.T) {
	evt := NewFailedEvent(uuid.NewV7(), uuid.NewV7())
	assert.Equal(t, ActionFailed, evt.Action())
}

func TestCancelledEvent(t *testing.T) {
	evt := NewCancelledEvent(uuid.NewV7(), uuid.NewV7())
	assert.Equal(t, ActionCancelled, evt.Action())
}

func TestRetryingEvent(t *testing.T) {
	evt := NewRetryingEvent(uuid.NewV7(), uuid.NewV7(), "ft-new")
	assert.Equal(t, ActionRetrying, evt.Action())
	assert.Equal(t, "ft-new", evt.NewFlowTaskId())
}
