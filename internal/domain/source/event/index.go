package event

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const IndexTopic = "inner.gonotelm.source.indexed"

type IndexEvent struct {
	event.BaseInnerEvent

	sourceId   valobj.Id
	notebookId valobj.Id
}

func NewIndexEvent(sourceId, notebookId valobj.Id) *IndexEvent {
	return &IndexEvent{
		sourceId:   sourceId,
		notebookId: notebookId,
	}
}

func (e *IndexEvent) SourceId() valobj.Id {
	return e.sourceId
}

func (e *IndexEvent) NotebookId() valobj.Id {
	return e.notebookId
}

func (e *IndexEvent) Topic() string {
	return IndexTopic
}

func (e *IndexEvent) Key() string {
	return e.sourceId.String()
}

func (e *IndexEvent) Value() any {
	return e
}
