package propagation

import (
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
)

// KafkaHeaderCarrier 是基于 kafka.Header 列表的 propagation.TextMapCarrier 实现。
type KafkaHeaderCarrier struct {
	headers []kafka.Header
}

func NewKafkaHeaderCarrier(headers []kafka.Header) *KafkaHeaderCarrier {
	return &KafkaHeaderCarrier{headers: headers}
}

var _ propagation.TextMapCarrier = (*KafkaHeaderCarrier)(nil)

// Headers 返回注入后的完整 header 列表。
func (c *KafkaHeaderCarrier) Headers() []kafka.Header {
	return c.headers
}

// Get returns the value associated with the passed key.
func (c *KafkaHeaderCarrier) Get(key string) string {
	for _, h := range c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

// Set stores the key-value pair.
func (c *KafkaHeaderCarrier) Set(key, value string) {
	c.headers = append(c.headers, kafka.Header{
		Key:   key,
		Value: []byte(value),
	})
}

// Keys lists the keys stored in this carrier.
func (c *KafkaHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for _, h := range c.headers {
		keys = append(keys, h.Key)
	}
	return keys
}
