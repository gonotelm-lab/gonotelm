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
