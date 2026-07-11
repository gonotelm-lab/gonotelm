package conf

import "time"

type FlowConfig struct {
	Addr        string        `toml:"addr"`
	Namespace   string        `toml:"namespace"`
	MaxRetry    int           `toml:"maxRetry"`
	DialTimeout time.Duration `toml:"dialTimeout"`
}