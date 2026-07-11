package conf

import "time"

type WorkerConfig struct {
	MaxConcurrency int           `toml:"maxConcurrency"`
	Heartbeat      time.Duration `toml:"heartbeat"`
}
