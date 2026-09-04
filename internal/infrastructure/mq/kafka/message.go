package kafka

import (
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"

	"github.com/segmentio/kafka-go"
)

type KafkaMessage struct {
	topic      string
	key, value []byte
	headers    []kafka.Header
}

var _ mq.Message = (*KafkaMessage)(nil)

func (m *KafkaMessage) Topic() string {
	return m.topic
}

func (m *KafkaMessage) Key() []byte {
	return m.key
}

func (m *KafkaMessage) Value() []byte {
	return m.value
}

func (m *KafkaMessage) Headers() []mq.MessageHeader {
	return fromKafkaHeaders(m.headers)
}
