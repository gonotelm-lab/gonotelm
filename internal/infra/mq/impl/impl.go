package impl

import (
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/infra/mq"
	"github.com/gonotelm-lab/gonotelm/internal/infra/mq/impl/kafka"
)

type Type string

const (
	Kafka Type = "kafka"
)

func New(cfg *Config) (*mq.MQ, error) {
	if cfg == nil {
		return nil, fmt.Errorf("mq config is nil")
	}

	switch cfg.Type {
	case Kafka:
		kafkaCfg := cfg.Kafka
		return &mq.MQ{
			NewProducer: func() mq.Producer {
				return kafka.NewProducer(kafka.ProducerConfig{
					Brokers:  kafkaCfg.Brokers,
					Username: kafkaCfg.Username,
					Password: kafkaCfg.Password,
				})
			},
			NewConsumer: func(topic, groupID string) mq.Consumer {
				return kafka.NewConsumer(kafka.ConsumerConfig{
					Brokers:        kafkaCfg.Brokers,
					GroupID:        groupID,
					Topic:          topic,
					QueueCapacity:  kafkaCfg.ConsumerQueueCapacity,
					CommitInterval: kafkaCfg.ConsumerCommitInterval,
					Username:       kafkaCfg.Username,
					Password:       kafkaCfg.Password,
				})
			},
		}, nil
	default:
		return nil, fmt.Errorf("impl type %s is not supported", cfg.Type)
	}
}
