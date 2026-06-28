package source

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const (
	PreparationTopic = "gonotelm.source.preparation"

	PreparationRetryHeaderKey   = "x-source-prepare-retry"
	PreparationRetryHeaderValue = "true"
)

type PreparationEvent struct {
	Id         valobj.Id    `json:"id"`
	NotebookId valobj.Id    `json:"notebook_id"`
	Kind       SourceKind   `json:"kind"`
	Status     SourceStatus `json:"status"`
	UserId     string       `json:"user_id"`

	isRetry bool
}

func (e *PreparationEvent) Category() event.Category {
	return event.CategoryOuter
}

func (e *PreparationEvent) Topic() string {
	return PreparationTopic
}

func (e *PreparationEvent) Key() string {
	return e.Id.String()
}

func (e *PreparationEvent) Value() any {
	return e
}

func (e *PreparationEvent) Headers() []event.Header {
	if !e.isRetry {
		return nil
	}

	return []event.Header{{
		Key:   PreparationRetryHeaderKey,
		Value: []byte(PreparationRetryHeaderValue),
	}}
}

func NewPreparationEvent(
	id valobj.Id,
	notebookId valobj.Id,
	kind SourceKind,
	status SourceStatus,
	userId string,
	isRetry bool,
) *PreparationEvent {
	return &PreparationEvent{
		Id:         id,
		NotebookId: notebookId,
		Kind:       kind,
		Status:     status,
		UserId:     userId,
		isRetry:    isRetry,
	}
}

const (
	DeleteTopic = "inner.gonotelm.source.deleted"
)

type DeleteEvent struct {
	Id         valobj.Id
	NotebookId valobj.Id
}

func NewDeleteEvent(id valobj.Id, notebookId valobj.Id) event.Event {
	return &DeleteEvent{
		Id:         id,
		NotebookId: notebookId,
	}
}

func (e *DeleteEvent) Category() event.Category {
	return event.CategoryOuter
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

func (e *DeleteEvent) Headers() []event.Header {
	return nil
}
