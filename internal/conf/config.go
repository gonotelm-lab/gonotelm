package conf

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	chat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	embedding "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	rerank "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	storageimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"github.com/BurntSushi/toml"
	"github.com/a8m/envsubst"
)

var (
	global        *Config
	setGlobalOnce sync.Once
)

type Config struct {
	DeployEnv string `toml:"deployEnv"`

	Logic LogicConfig `toml:"logic"`

	Api        ApiConfig              `toml:"api"`
	Database   DatabaseConfig         `toml:"database"`
	Redis      cache.RedisCacheConfig `toml:"redis"`
	VectorDB   vectordb.Config        `toml:"vectorDb"`
	Storage    storageimpl.StorageTypeConfig     `toml:"storage"`
	MsgQueue   mqimpl.Config          `toml:"msgQueue"`
	Embedding  embedding.EmbeddingConfig  `toml:"embedding"`
	Rerank     rerank.RerankConfig        `toml:"rerank"`
	Text2Image text2image.Text2ImageConfig `toml:"text2image"`
	Logging    LoggingConfig          `toml:"logging"`
	Chunking   ChunkingConfig         `toml:"chunking"`
	Provider   chat.ProviderConfig    `toml:"provider"`
}

func (c *Config) IsDev() bool {
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

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q failed: %w", path, err)
	}

	expanded, err := envsubst.String(string(raw))
	if err != nil {
		return nil, fmt.Errorf("expand env in config file %q failed: %w", path, err)
	}

	cfg := &Config{}
	if _, err := toml.Decode(expanded, cfg); err != nil {
		return nil, fmt.Errorf("decode config file %q failed: %w", path, err)
	}

	if cfg.Storage.Type == "" {
		cfg.Storage.Type = storageimpl.Minio
	}
	if cfg.MsgQueue.Type == "" {
		cfg.MsgQueue.Type = mqimpl.Kafka
	}
	if cfg.Embedding.Type == "" {
		cfg.Embedding.Type = embedding.EmbeddingDashScope
	}
	if cfg.Rerank.Type == "" {
		cfg.Rerank.Type = rerank.RerankDashScope
	}
	if cfg.Text2Image.Type == "" {
		cfg.Text2Image.Type = text2image.Text2ImageDashScope
	}
	if cfg.Embedding.BatchSize <= 0 {
		cfg.Embedding.BatchSize = 10
	}
	if cfg.Embedding.MaxConcurrency <= 0 {
		cfg.Embedding.MaxConcurrency = 4
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "debug"
	}

	global = cfg

	return cfg, nil
}

func Global() *Config {
	return global
}

func SetGlobal(cfg *Config) {
	setGlobalOnce.Do(func() {
		global = cfg
	})
}

func (c *Config) SQLConfig() *sql.Config {
	return c.Database.ToSQLConfig()
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

type ChunkingConfig struct {
	Size        int `toml:"size"`
	OverlapSize int `toml:"overlapSize"`
}
