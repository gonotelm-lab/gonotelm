package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type Base struct {
	Id         valobj.Id   `json:"id"`
	CreateTime valobj.Time `json:"create_time"`
	UpdateTime valobj.Time `json:"update_time"`

	deleted bool          `json:"-"`
	events  []event.Event `json:"-"`
}

func (e *Base) IsDeleted() bool {
	return e.deleted
}

func (e *Base) Delete() {
	e.deleted = true
}

func (e *Base) PullEvents() []event.Event {
	events := e.events
	e.events = nil
	return events
}

func (e *Base) AddEvent(event event.Event) {
	e.events = append(e.events, event)
}

func NewBase() Base {
	return Base{
		Id:         valobj.NewId(),
		CreateTime: valobj.NewTime(),
		UpdateTime: valobj.NewTime(),
	}
}
