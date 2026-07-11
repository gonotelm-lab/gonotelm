package event

type Category string

const (
	CategoryInner Category = "inner"
	CategoryOuter Category = "outer"
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

type BaseInnerEvent struct{}

func (e *BaseInnerEvent) Category() Category {
	return CategoryInner
}

func (e *BaseInnerEvent) Topic() string {
	return ""
}

func (e *BaseInnerEvent) Key() string {
	return ""
}

func (e *BaseInnerEvent) Value() any {
	return nil
}

func (e *BaseInnerEvent) Headers() []Header {
	return nil
}
