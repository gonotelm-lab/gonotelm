package impl

import "time"

type Config struct {
	Type Type `toml:"type"`

	Kafka KafkaConfig `toml:"kafka"`
}

type KafkaConfig struct {
	Brokers                []string      `toml:"brokers"`
	Username               string        `toml:"username"`
	Password               string        `toml:"password"`
	ConsumerQueueCapacity  int           `toml:"consumerQueueCapacity"`
	ConsumerCommitInterval time.Duration `toml:"consumerCommitInterval"`
}
