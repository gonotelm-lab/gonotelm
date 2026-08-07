package propagation

import (
	"testing"
)

func TestKafkaHeaderCarrier(t *testing.T) {
	c := NewKafkaHeaderCarrier(nil)
	c.Set("traceparent", "00-abc-xyz-01")
	c.Set("baggage", "k=v")

	if got := c.Get("traceparent"); got != "00-abc-xyz-01" {
		t.Fatalf("Get traceparent: %s", got)
	}
	if got := c.Get("missing"); got != "" {
		t.Fatalf("Get missing should be empty: %s", got)
	}

	keys := c.Keys()
	if len(keys) != 2 {
		t.Fatalf("unexpected keys: %v", keys)
	}

	// 基于 kafka.Header 的 Get 应命中
	if got := string(c.Headers()[0].Value); got != "00-abc-xyz-01" {
		t.Fatalf("header value: %s", got)
	}
}
