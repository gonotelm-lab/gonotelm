package mq

import (
	"context"
	"time"
)

type MessageHeader struct {
	Key   string
	Value []byte
}

type Message interface {
	Topic() string

	Key() []byte

	Value() []byte

	Headers() []MessageHeader
}

type ProducerSendRequest struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers []MessageHeader
}

type Producer interface {
	Send(ctx context.Context, req *ProducerSendRequest) error
	Close(ctx context.Context) error
}

type MessageHandler func(ctx context.Context, msg Message) error

type Consumer interface {
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
	Close(ctx context.Context) error
}

type ProducerFactory func() Producer

type ConsumerFactory func(topic, groupID string) Consumer

type MQ struct {
	NewProducer ProducerFactory
	NewConsumer ConsumerFactory
}

type Type string

const (
	Kafka Type = "kafka"
)

type Config struct {
	Type  Type        `toml:"type"`
	Kafka KafkaConfig `toml:"kafka"`
}

type KafkaConfig struct {
	Brokers                []string      `toml:"brokers"`
	Username               string        `toml:"username"`
	Password               string        `toml:"password"`
	ConsumerQueueCapacity  int           `toml:"consumerQueueCapacity"`
	ConsumerCommitInterval time.Duration `toml:"consumerCommitInterval"`
}
