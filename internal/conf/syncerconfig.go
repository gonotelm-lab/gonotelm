package conf

import "time"

type SyncerConfig struct {
	PerTaskInterval time.Duration `toml:"perTaskInterval"`
	GlobalInterval  time.Duration `toml:"globalInterval"`
	GlobalBatchSize int           `toml:"globalBatchSize"`
}
