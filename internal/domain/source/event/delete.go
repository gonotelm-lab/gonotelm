package event

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const DeleteTopic = "inner.gonotelm.source.deleted"

type DeleteEvent struct {
	event.BaseInnerEvent

	sourceId   valobj.Id
	notebookId valobj.Id
	objectKeys []string
}

func NewDeleteEvent(sourceId, notebookId valobj.Id, objectKeys []string) *DeleteEvent {
	return &DeleteEvent{
		sourceId:   sourceId,
		notebookId: notebookId,
		objectKeys: objectKeys,
	}
}

func (e *DeleteEvent) SourceId() valobj.Id {
	return e.sourceId
}

func (e *DeleteEvent) NotebookId() valobj.Id {
	return e.notebookId
}

func (e *DeleteEvent) ObjectStoreKeys() []string {
	return e.objectKeys
}

func (e *DeleteEvent) Topic() string {
	return DeleteTopic
}

func (e *DeleteEvent) Key() string {
	return e.sourceId.String()
}

func (e *DeleteEvent) Value() any {
	return e
}
