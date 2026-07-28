package conf

import (
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
)

var (
	sourceJobGlobal  *SourceJobConfig
	setSourceJobOnce sync.Once
)

// SourceJobConfig is the config for cmd/sourcejob (source preparation consumer).
type SourceJobConfig struct {
	shared.InfraConfig

	DeployEnv string               `toml:"deployEnv"`
	Source    SourceConfig         `toml:"source"`
	Logging   shared.LoggingConfig `toml:"logging"`
	Chunking  ChunkingConfig       `toml:"chunking"`
}

func (c *SourceJobConfig) IsDev() bool {
	return shared.IsDevEnv(c.DeployEnv)
}

func LoadSourceJobConfig(path string) (*SourceJobConfig, error) {
	cfg := &SourceJobConfig{}
	if err := loadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	sourceJobGlobal = cfg
	return cfg, nil
}

func (c *SourceJobConfig) applyDefaults() {
	c.InfraConfig.ApplyDefaults()
	c.Logging.ApplyDefaults()

	if c.MsgQueue.Type == "" {
		c.MsgQueue.Type = mqimpl.Kafka
	}
}

func SourceJobGlobal() *SourceJobConfig {
	return sourceJobGlobal
}

func SetSourceJobGlobal(cfg *SourceJobConfig) {
	setSourceJobOnce.Do(func() {
		sourceJobGlobal = cfg
	})
}
