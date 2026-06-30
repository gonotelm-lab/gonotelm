package event

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const DeleteTopic = "inner.gonotelm.source.deleted"

type DeleteEvent struct {
	event.BaseInnerEvent

	Id         valobj.Id
	NotebookId valobj.Id
}

func NewDeleteEvent(id valobj.Id, notebookId valobj.Id) event.Event {
	return &DeleteEvent{
		Id:         id,
		NotebookId: notebookId,
	}
}

func (e *DeleteEvent) Topic() string {
	return DeleteTopic
}

func (e *DeleteEvent) Key() string {
	return e.Id.String()
}

func (e *DeleteEvent) Value() any {
	return e
}
