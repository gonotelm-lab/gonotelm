package messagingconv

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func attrMap(attrs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	m := make(map[attribute.Key]attribute.Value, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

func TestSendAttributes(t *testing.T) {
	attrs := attrMap(SendAttributes("orders", []byte("key-1"), SendOptions{}))

	want := map[attribute.Key]string{
		"messaging.system":            "kafka",
		"messaging.operation.name":    "send",
		"messaging.operation.type":    "send",
		"messaging.destination.name":  "orders",
		"messaging.kafka.message.key": "key-1",
	}
	for k, wantV := range want {
		if got := attrs[k]; got.AsString() != wantV {
			t.Errorf("attribute %s: got %q want %q", k, got.AsString(), wantV)
		}
	}
	if got := attrs["messaging.kafka.message.tombstone"]; got.Type() != attribute.EMPTY {
		t.Errorf("tombstone must not be set for zero options, got %v", got)
	}
}

func TestSendAttributesTombstone(t *testing.T) {
	attrs := attrMap(SendAttributes("orders", []byte("key-1"), SendOptions{Tombstone: true}))
	if got := attrs["messaging.kafka.message.tombstone"].AsBool(); !got {
		t.Errorf("messaging.kafka.message.tombstone: got %v want true", got)
	}
}

func TestSendAttributesNilKey(t *testing.T) {
	attrs := attrMap(SendAttributes("orders", nil, SendOptions{}))
	if got := attrs["messaging.kafka.message.key"]; got.Type() != attribute.EMPTY {
		t.Errorf("message key should not be set when nil, got %v", got)
	}
	if got := attrs["messaging.destination.name"].AsString(); got != "orders" {
		t.Errorf("destination: got %q want %q", got, "orders")
	}
}

func TestSpanNames(t *testing.T) {
	if got := SendSpanName("orders"); got != "send orders" {
		t.Errorf("SendSpanName: got %q want %q", got, "send orders")
	}
	if got := SendSpanName(""); got != "send" {
		t.Errorf("SendSpanName empty: got %q want %q", got, "send")
	}
	if got := ProcessSpanName("orders"); got != "process orders" {
		t.Errorf("ProcessSpanName: got %q want %q", got, "process orders")
	}
	if got := ProcessSpanName(""); got != "process" {
		t.Errorf("ProcessSpanName empty: got %q want %q", got, "process")
	}
}

func TestProcessAttributes(t *testing.T) {
	attrs := attrMap(ProcessAttributes("orders", []byte("key-2"), ProcessOptions{
		GroupName: "my-group",
		Partition: -1,
		Offset:    -1,
	}))

	want := map[attribute.Key]string{
		"messaging.system":            "kafka",
		"messaging.operation.name":    "process",
		"messaging.operation.type":    "process",
		"messaging.destination.name":  "orders",
		"messaging.kafka.message.key": "key-2",
	}
	for k, wantV := range want {
		if got := attrs[k]; got.AsString() != wantV {
			t.Errorf("attribute %s: got %q want %q", k, got.AsString(), wantV)
		}
	}
	if got := attrs["messaging.consumer.group.name"].AsString(); got != "my-group" {
		t.Errorf("messaging.consumer.group.name: got %q want %q", got, "my-group")
	}
	// partition/offset < 0 are treated as unset
	for _, k := range []string{"messaging.destination.partition.id", "messaging.kafka.offset", "messaging.kafka.message.tombstone"} {
		if got := attrs[attribute.Key(k)]; got.Type() != attribute.EMPTY {
			t.Errorf("attribute %s must not be set, got %v", k, got)
		}
	}
}

func TestProcessAttributesWithOptions(t *testing.T) {
	attrs := attrMap(ProcessAttributes("orders", []byte("key-2"), ProcessOptions{
		GroupName: "my-group",
		Partition: 3,
		Offset:    42,
		Tombstone: true,
	}))

	if got := attrs["messaging.consumer.group.name"].AsString(); got != "my-group" {
		t.Errorf("messaging.consumer.group.name: got %q want %q", got, "my-group")
	}
	if got := attrs["messaging.destination.partition.id"].AsString(); got != "3" {
		t.Errorf("messaging.destination.partition.id: got %q want %q", got, "3")
	}
	if got := attrs["messaging.kafka.offset"].AsInt64(); got != 42 {
		t.Errorf("messaging.kafka.offset: got %d want 42", got)
	}
	if got := attrs["messaging.kafka.message.tombstone"].AsBool(); !got {
		t.Errorf("messaging.kafka.message.tombstone: got %v want true", got)
	}
}

func TestProcessAttributesNilKey(t *testing.T) {
	attrs := attrMap(ProcessAttributes("orders", nil, ProcessOptions{}))
	if got := attrs["messaging.kafka.message.key"]; got.Type() != attribute.EMPTY {
		t.Errorf("message key should not be set when nil, got %v", got)
	}
	if got := attrs["messaging.destination.name"].AsString(); got != "orders" {
		t.Errorf("destination: got %q want %q", got, "orders")
	}
}
