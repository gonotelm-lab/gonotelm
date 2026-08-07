package messagingconv

import (
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// ProcessOptions carries optional consumer attributes. Zero values are omitted.
type ProcessOptions struct {
	GroupName string // messaging.consumer.group.name
	Partition int    // messaging.destination.partition.id; set when >= 0
	Offset    int64  // messaging.kafka.offset; set when >= 0
	Tombstone bool   // messaging.kafka.message.tombstone; set when true
}

// SendOptions carries optional producer attributes. Zero values are omitted.
type SendOptions struct {
	Tombstone bool // messaging.kafka.message.tombstone; set when true
}

// SendSpanName returns the span name "{operation.name} {destination}".
// The destination is omitted when empty.
func SendSpanName(destination string) string {
	return operationSpanName("send", destination)
}

// ProcessSpanName returns the span name "{operation.name} {destination}".
// The destination is omitted when empty.
func ProcessSpanName(destination string) string {
	return operationSpanName("process", destination)
}

// SendAttributes returns producer span attributes (messaging.operation.type=send).
func SendAttributes(destination string, key []byte, o SendOptions) []attribute.KeyValue {
	attrs := operationAttributes("send", destination, key)
	if o.Tombstone {
		attrs = append(attrs, semconv.MessagingKafkaMessageTombstoneKey.Bool(true))
	}
	return attrs
}

// ProcessAttributes returns consumer span attributes (messaging.operation.type=process).
func ProcessAttributes(destination string, key []byte, o ProcessOptions) []attribute.KeyValue {
	attrs := operationAttributes("process", destination, key)
	if o.GroupName != "" {
		attrs = append(attrs, semconv.MessagingConsumerGroupNameKey.String(o.GroupName))
	}
	if o.Partition >= 0 {
		attrs = append(attrs, semconv.MessagingDestinationPartitionIDKey.String(strconv.Itoa(o.Partition)))
	}
	if o.Offset >= 0 {
		attrs = append(attrs, semconv.MessagingKafkaOffsetKey.Int64(o.Offset))
	}
	if o.Tombstone {
		attrs = append(attrs, semconv.MessagingKafkaMessageTombstoneKey.Bool(true))
	}
	return attrs
}

func operationAttributes(operation, destination string, key []byte) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.MessagingSystemKafka,
		semconv.MessagingOperationNameKey.String(operation),
		semconv.MessagingOperationTypeKey.String(operation),
		semconv.MessagingDestinationNameKey.String(destination),
	}
	if len(key) > 0 {
		attrs = append(attrs, semconv.MessagingKafkaMessageKeyKey.String(string(key)))
	}
	return attrs
}

func operationSpanName(op, destination string) string {
	if destination == "" {
		return op
	}
	return op + " " + destination
}
