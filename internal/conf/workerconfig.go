package conf

import "time"

type WorkerConfig struct {
	Name           string        `toml:"name"`
	MaxConcurrency int           `toml:"maxConcurrency"`
	Heartbeat      time.Duration `toml:"heartbeat"`
	TaskTypes      []string      `toml:"taskTypes"`
}