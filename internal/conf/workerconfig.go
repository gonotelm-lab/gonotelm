package conf

import "time"

type WorkerPoolConfig struct {
	MaxConcurrency int           `toml:"maxConcurrency"`
	Heartbeat      time.Duration `toml:"heartbeat"`
}
