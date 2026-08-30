package conf

import (
	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

var sourceJobGlobal *SourceJobConfig

// SourceJobConfig is the config for cmd/sourcejob (source preparation consumer).
type SourceJobConfig struct {
	shared.InfraConfig

	DeployEnv string               `toml:"deployEnv"`
	Source    SourceConfig         `toml:"source"`
	Logging   shared.LoggingConfig `toml:"logging"`
	Chunking  ChunkingConfig       `toml:"chunking"`
	OtelTrace trace.Config         `toml:"otelTrace"`
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
	c.Init()
	c.Logging.ApplyDefaults()

	if c.MsgQueue.Type == "" {
		c.MsgQueue.Type = mqimpl.Kafka
	}
}

func SourceJobGlobal() *SourceJobConfig {
	return sourceJobGlobal
}
