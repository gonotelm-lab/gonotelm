package event

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const IndexTopic = "inner.gonotelm.source.indexed"

type IndexEvent struct {
	event.BaseInnerEvent

	Id         valobj.Id
	NotebookId valobj.Id
}

func NewIndexEvent(id valobj.Id, notebookId valobj.Id) event.Event {
	return &IndexEvent{
		Id:         id,
		NotebookId: notebookId,
	}
}

func (e *IndexEvent) Topic() string {
	return IndexTopic
}

func (e *IndexEvent) Key() string {
	return e.Id.String()
}

func (e *IndexEvent) Value() any {
	return e
}

