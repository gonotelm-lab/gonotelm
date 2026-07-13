package event

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const TopicArtifactEvent = "gonotelm.artifact.event"

type Action string

const (
	ActionCompleted Action = "artifact.completed"
	ActionFailed    Action = "artifact.failed"
	ActionCancelled Action = "artifact.cancelled"
	ActionRetrying  Action = "artifact.retrying"
)

type Event struct {
	event.BaseInnerEvent

	artifactId    valobj.Id
	notebookId    valobj.Id
	action        Action
	newFlowTaskId string
}

func NewCompletedEvent(artifactId, notebookId valobj.Id) *Event {
	return &Event{
		artifactId: artifactId,
		notebookId: notebookId,
		action:     ActionCompleted,
	}
}

func NewFailedEvent(artifactId, notebookId valobj.Id) *Event {
	return &Event{
		artifactId: artifactId,
		notebookId: notebookId,
		action:     ActionFailed,
	}
}

func NewCancelledEvent(artifactId, notebookId valobj.Id) *Event {
	return &Event{
		artifactId: artifactId,
		notebookId: notebookId,
		action:     ActionCancelled,
	}
}

func NewRetryingEvent(artifactId, notebookId valobj.Id, newFlowTaskId string) *Event {
	return &Event{
		artifactId:    artifactId,
		notebookId:    notebookId,
		action:        ActionRetrying,
		newFlowTaskId: newFlowTaskId,
	}
}

func (e *Event) ArtifactId() valobj.Id { return e.artifactId }
func (e *Event) NotebookId() valobj.Id { return e.notebookId }
func (e *Event) Action() Action        { return e.action }
func (e *Event) NewFlowTaskId() string { return e.newFlowTaskId }

func (e *Event) Topic() string { return TopicArtifactEvent }

func (e *Event) Key() string { return e.artifactId.String() }

func (e *Event) Value() any { return e }
