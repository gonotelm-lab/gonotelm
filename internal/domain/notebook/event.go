package notebook

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const (
	TopicNotebookEvent = "gonotelm.notebook.event"
)

type EventAction string

const (
	EventActionCreate EventAction = "notebook.created"
	EventActionDelete EventAction = "notebook.deleted"
)

type Event struct {
	notebookId valobj.Id
	action     EventAction
}

func (e *Event) NotebookId() valobj.Id {
	return e.notebookId
}

func (e *Event) Action() EventAction {
	return e.action
}

func (e *Event) Category() event.Category {
	return event.CategoryInner // processed in-process via CompositeEventBus.Inner
}

func (e *Event) Topic() string {
	return TopicNotebookEvent
}

func (e *Event) Key() string {
	return e.notebookId.String()
}

func (e *Event) Value() any {
	return e
}

func (e *Event) Headers() []event.Header {
	return nil
}

func NewEvent(notebookId valobj.Id, action EventAction) *Event {
	return &Event{
		notebookId: notebookId,
		action:     action,
	}
}
