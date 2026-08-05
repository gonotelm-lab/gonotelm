package propagation

import (
	"github.com/cloudwego/hertz/pkg/protocol"
	"go.opentelemetry.io/otel/propagation"
)

type HertzRequestHeaderCarrier struct {
	rh *protocol.RequestHeader
}

func NewHertzRequestHeaderCarrier(rh *protocol.RequestHeader) *HertzRequestHeaderCarrier {
	return &HertzRequestHeaderCarrier{rh: rh}
}

var _ propagation.TextMapCarrier = &HertzRequestHeaderCarrier{}

// Get returns the value associated with the passed key.
func (c *HertzRequestHeaderCarrier) Get(key string) string {
	return c.rh.Get(key)
}

// Set stores the key-value pair.
func (c *HertzRequestHeaderCarrier) Set(key, value string) {
	c.rh.Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (c *HertzRequestHeaderCarrier) Keys() []string {
	keys := make([]string, 0, c.rh.Len())
	c.rh.VisitAll(func(key, value []byte) {
		keys = append(keys, string(key))
	})

	return keys
}

type HertzResponseHeaderCarrier struct {
	rh *protocol.ResponseHeader
}

func NewHertzResponseHeaderCarrier(rh *protocol.ResponseHeader) *HertzResponseHeaderCarrier {
	return &HertzResponseHeaderCarrier{rh: rh}
}

var _ propagation.TextMapCarrier = &HertzResponseHeaderCarrier{}

// Get returns the value associated with the passed key.
func (c *HertzResponseHeaderCarrier) Get(key string) string {
	return string(c.rh.Peek(key))
}

// Set stores the key-value pair.
func (c *HertzResponseHeaderCarrier) Set(key, value string) {
	c.rh.Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (c *HertzResponseHeaderCarrier) Keys() []string {
	keys := make([]string, 0)
	c.rh.VisitAll(func(key, value []byte) {
		keys = append(keys, string(key))
	})

	return keys
}
