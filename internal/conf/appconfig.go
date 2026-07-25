package conf

import (
	"fmt"
	"os"
	"sync"
	"time"

	rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/rerank"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"

	"github.com/BurntSushi/toml"
	"github.com/a8m/envsubst"
)

var (
	appGlobal     *AppConfig
	workerGlobal  *WorkerConfig
	setAppOnce    sync.Once
	setWorkerOnce sync.Once
)

type AppConfig struct {
	InfraConfig

	DeployEnv string `toml:"deployEnv"`

	Api      ApiConfig           `toml:"api"`
	Chat     ChatConfig          `toml:"chat"`
	Source   SourceConfig        `toml:"source"`
	Rerank   rerank.RerankConfig `toml:"rerank"`
	Logging  LoggingConfig       `toml:"logging"`
	Chunking ChunkingConfig      `toml:"chunking"`
	Flow     FlowConfig          `toml:"flow"`
	Syncer   SyncerConfig        `toml:"syncer"`
}

type SyncerConfig struct {
	PerTaskInterval time.Duration `toml:"perTaskInterval"`
	GlobalInterval  time.Duration `toml:"globalInterval"`
	GlobalBatchSize int           `toml:"globalBatchSize"`
}

func (c *AppConfig) IsDev() bool {
	return IsDevEnv(c.DeployEnv)
}

func (c *WorkerConfig) IsDev() bool {
	return IsDevEnv(c.DeployEnv)
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

func LoadAppConfig(path string) (*AppConfig, error) {
	cfg := &AppConfig{}
	if err := loadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	appGlobal = cfg
	return cfg, nil
}

func LoadWorkerConfig(path string) (*WorkerConfig, error) {
	cfg := &WorkerConfig{}
	if err := loadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	workerGlobal = cfg
	return cfg, nil
}

func (c *AppConfig) applyDefaults() {
	c.InfraConfig.applyDefaults()
	c.Logging.applyDefaults()
	c.Flow.applyDefaults()

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

func (c *WorkerConfig) applyDefaults() {
	c.InfraConfig.applyDefaults()
	c.Logging.applyDefaults()
	c.Flow.applyDefaults()

	if c.Worker.MaxConcurrency <= 0 {
		c.Worker.MaxConcurrency = 4
	}
	if c.Worker.Heartbeat == 0 {
		c.Worker.Heartbeat = 5 * time.Second
	}
}

func AppGlobal() *AppConfig {
	return appGlobal
}

func SetAppGlobal(cfg *AppConfig) {
	setAppOnce.Do(func() {
		appGlobal = cfg
	})
}

func WorkerGlobal() *WorkerConfig {
	return workerGlobal
}

func SetWorkerGlobal(cfg *WorkerConfig) {
	setWorkerOnce.Do(func() {
		workerGlobal = cfg
	})
}
