package event

type Category string

const (
	CategoryInProcess    Category = "inprocess"
	CategoryInterProcess Category = "interprocess"
)

type Header struct {
	Key   string
	Value []byte
}

type Event interface {
	Category() Category
	Topic() string
	Key() string
	Value() any
	Headers() []Header
}

type BaseInProcessEvent struct{}

func (e *BaseInProcessEvent) Category() Category {
	return CategoryInProcess
}

func (e *BaseInProcessEvent) Topic() string {
	return ""
}

func (e *BaseInProcessEvent) Key() string {
	return ""
}

func (e *BaseInProcessEvent) Value() any {
	return nil
}

func (e *BaseInProcessEvent) Headers() []Header {
	return nil
}
