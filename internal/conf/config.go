package conf

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	chat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	embedding "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/rerank"
	text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	storageimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"github.com/BurntSushi/toml"
	"github.com/a8m/envsubst"
)

var (
	appGlobal        *AppConfig
	workerGlobal     *WorkerConfig
	setAppOnce    sync.Once
	setWorkerOnce sync.Once
)

type InfraConfig struct {
	Database   DatabaseConfig                `toml:"database"`
	VectorDB   vectordb.Config               `toml:"vectorDb"`
	Storage    storageimpl.StorageTypeConfig `toml:"storage"`
	Provider   chat.ProviderConfig           `toml:"provider"`
	Embedding  embedding.EmbeddingConfig     `toml:"embedding"`
	Text2Image text2image.Text2ImageConfig   `toml:"text2image"`

	Redis    cache.RedisCacheConfig `toml:"redis"`
	MsgQueue mqimpl.Config          `toml:"msgQueue"`
}

type AppConfig struct {
	InfraConfig

	DeployEnv string `toml:"deployEnv"`

	Api      ApiConfig          `toml:"api"`
	Chat     ChatConfig         `toml:"chat"`
	Source   SourceConfig       `toml:"source"`
	Rerank   rerank.RerankConfig `toml:"rerank"`
	Logging  LoggingConfig      `toml:"logging"`
	Chunking ChunkingConfig     `toml:"chunking"`
	Flow     FlowConfig         `toml:"flow"`
	Worker   WorkerPoolConfig   `toml:"worker"`
	Syncer   SyncerConfig       `toml:"syncer"`
}

type WorkerConfig struct {
	InfraConfig

	DeployEnv string          `toml:"deployEnv"`
	Studio    StudioConfig    `toml:"studio"`
	Logging   LoggingConfig   `toml:"logging"`
	Flow      FlowConfig      `toml:"flow"`
	Worker    WorkerPoolConfig `toml:"worker"`
}

func (c *AppConfig) IsDev() bool {
	return c.DeployEnv == "dev"
}

func (c *WorkerConfig) IsDev() bool {
	return c.DeployEnv == "dev"
}

type ApiConfig struct {
	Port            int           `toml:"port"`
	ExitWaitTimeout time.Duration `toml:"exitWaitTimeout"`
}

func (c *ApiConfig) HostPort() string {
	return fmt.Sprintf(":%d", c.Port)
}

type DatabaseConfig struct {
	Type     string `toml:"type"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DBName   string `toml:"dbName"`
}

type LoggingConfig struct {
	Level string `toml:"level"`
}

type ChunkingConfig struct {
	Size        int `toml:"size"`
	OverlapSize int `toml:"overlapSize"`
}

func (d *DatabaseConfig) ToSQLConfig() *sql.Config {
	return &sql.Config{
		Host:     d.Host,
		Port:     d.Port,
		User:     d.User,
		Password: d.Password,
		DbName:   d.DBName,
	}
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
	if c.Storage.Type == "" {
		c.Storage.Type = storageimpl.Minio
	}
	if c.MsgQueue.Type == "" {
		c.MsgQueue.Type = mqimpl.Kafka
	}
	if c.Embedding.Type == "" {
		c.Embedding.Type = embedding.EmbeddingDashScope
	}
	if c.Rerank.Type == "" {
		c.Rerank.Type = rerank.RerankDashScope
	}
	if c.Text2Image.Type == "" {
		c.Text2Image.Type = text2image.Text2ImageDashScope
	}
	if c.Embedding.BatchSize <= 0 {
		c.Embedding.BatchSize = 10
	}
	if c.Embedding.MaxConcurrency <= 0 {
		c.Embedding.MaxConcurrency = 4
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "debug"
	}
	if c.Flow.MaxRetry <= 0 {
		c.Flow.MaxRetry = 3
	}
	if c.Flow.DialTimeout == 0 {
		c.Flow.DialTimeout = 5 * time.Second
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
	if c.Worker.MaxConcurrency <= 0 {
		c.Worker.MaxConcurrency = 4
	}
	if c.Worker.Heartbeat == 0 {
		c.Worker.Heartbeat = 5 * time.Second
	}
}

func (c *WorkerConfig) applyDefaults() {
	if c.Storage.Type == "" {
		c.Storage.Type = storageimpl.Minio
	}
	if c.Embedding.Type == "" {
		c.Embedding.Type = embedding.EmbeddingDashScope
	}
	if c.Text2Image.Type == "" {
		c.Text2Image.Type = text2image.Text2ImageDashScope
	}
	if c.Embedding.BatchSize <= 0 {
		c.Embedding.BatchSize = 10
	}
	if c.Embedding.MaxConcurrency <= 0 {
		c.Embedding.MaxConcurrency = 4
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "debug"
	}
	if c.Flow.MaxRetry <= 0 {
		c.Flow.MaxRetry = 3
	}
	if c.Flow.DialTimeout == 0 {
		c.Flow.DialTimeout = 5 * time.Second
	}
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

func (c *AppConfig) SQLConfig() *sql.Config {
	return c.Database.ToSQLConfig()
}

func (c *WorkerConfig) SQLConfig() *sql.Config {
	return c.Database.ToSQLConfig()
}
