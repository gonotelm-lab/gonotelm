package event

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
)

const (
	PreparationTopic = "gonotelm.source.preparation"

	PreparationRetryHeaderKey   = "x-source-prepare-retry"
	PreparationRetryHeaderValue = "true"
)

type PreparationEvent struct {
	Id         valobj.Id       `json:"id"`
	NotebookId valobj.Id       `json:"notebook_id"`
	Kind       vo.SourceKind   `json:"kind"`
	Status     vo.SourceStatus `json:"status"`
	UserId     string          `json:"user_id"`

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
	kind vo.SourceKind,
	status vo.SourceStatus,
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
