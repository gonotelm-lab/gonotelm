package conf

import (
	"fmt"
	"os"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/rerank"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"

	"github.com/BurntSushi/toml"
	"github.com/a8m/envsubst"
)

var notelmGlobal *NotelmConfig

type NotelmConfig struct {
	shared.InfraConfig

	DeployEnv string `toml:"deployEnv"`

	Api       ApiConfig            `toml:"api"`
	Chat      ChatConfig           `toml:"chat"`
	Source    SourceConfig         `toml:"source"`
	Rerank    rerank.RerankConfig  `toml:"rerank"`
	Logging   shared.LoggingConfig `toml:"logging"`
	Chunking  ChunkingConfig       `toml:"chunking"`
	Flow      shared.FlowConfig    `toml:"flow"`
	Syncer    SyncerConfig         `toml:"syncer"`
	OtelTrace trace.Config         `toml:"otelTrace"`
}

type SyncerConfig struct {
	PerTaskInterval time.Duration `toml:"perTaskInterval"`
	GlobalInterval  time.Duration `toml:"globalInterval"`
	GlobalBatchSize int           `toml:"globalBatchSize"`
}

func (c *NotelmConfig) IsDev() bool {
	return shared.IsDevEnv(c.DeployEnv)
}

func (c *WorkerConfig) IsDev() bool {
	return shared.IsDevEnv(c.DeployEnv)
}

type ApiConfig struct {
	Port            int           `toml:"port"`
	ExitWaitTimeout time.Duration `toml:"exitWaitTimeout"`
}

func (c *ApiConfig) HostPort() string {
	return fmt.Sprintf(":%d", c.Port)
}

type ChunkingConfig struct {
	Size        int `toml:"size"`
	OverlapSize int `toml:"overlapSize"`
}

func loadTOML(path string, cfg interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %q failed: %w", path, err)
	}

	expanded, err := envsubst.String(string(raw))
	if err != nil {
		return fmt.Errorf("expand env in config file %q failed: %w", path, err)
	}

	if _, err := toml.Decode(expanded, cfg); err != nil {
		return fmt.Errorf("decode config file %q failed: %w", path, err)
	}

	return nil
}

func LoadNotelmConfig(path string) (*NotelmConfig, error) {
	cfg := &NotelmConfig{}
	if err := loadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	notelmGlobal = cfg
	notelmGlobal.Source.init()

	return notelmGlobal, nil
}

func (c *NotelmConfig) applyDefaults() {
	c.ApplyDefaults()
	c.Logging.ApplyDefaults()
	c.Flow.ApplyDefaults()

	if c.MsgQueue.Type == "" {
		c.MsgQueue.Type = mqimpl.Kafka
	}
	if c.Syncer.PerTaskInterval == 0 {
		c.Syncer.PerTaskInterval = 2 * time.Second
	}
	if c.Syncer.GlobalInterval == 0 {
		c.Syncer.GlobalInterval = 5 * time.Second
	}
	if c.Syncer.GlobalBatchSize <= 0 {
		c.Syncer.GlobalBatchSize = 100
	}
}

func NotelmGlobal() *NotelmConfig {
	return notelmGlobal
}
