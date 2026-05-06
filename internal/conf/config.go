package conf

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/a8m/envsubst"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	embedimpl "github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infra/mq/impl"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	storageimpl "github.com/gonotelm-lab/gonotelm/internal/infra/storage/impl"
	vecimpl "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/impl"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"
)

var (
	global        *Config
	setGlobalOnce sync.Once
)

type Config struct {
	DeployEnv string `toml:"deployEnv"`

	Logic LogicConfig `toml:"logic"`

	Api       ApiConfig              `toml:"api"`
	Database  DatabaseConfig         `toml:"database"`
	Redis     cache.RedisCacheConfig `toml:"redis"`
	VectorDB  vecimpl.Config         `toml:"vectorDb"`
	Storage   StorageConfig          `toml:"storage"`
	MsgQueue  MsgQueueConfig         `toml:"msgQueue"`
	Embedding embedimpl.Config       `toml:"embedding"`
	Logging   LoggingConfig          `toml:"logging"`
	Chunking  ChunkingConfig         `toml:"chunking"`
	ChatModel chat.Config            `toml:"chatModel"`
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

type StorageConfig struct {
	Type  storageimpl.Type `toml:"type"`
	Minio MinioConfig      `toml:"minio"`
}

type MinioConfig struct {
	Endpoint      string        `toml:"endpoint"`
	AccessKey     string        `toml:"accessKey"`
	SecretKey     string        `toml:"secretKey"`
	Bucket        string        `toml:"bucket"`
	Region        string        `toml:"region"`
	Secure        bool          `toml:"secure"`
	PresignExpiry time.Duration `toml:"presignExpiry"`
}

type MsgQueueConfig struct {
	Type  mqimpl.Type `toml:"type"`
	Kafka KafkaConfig `toml:"kafka"`
}

type KafkaConfig struct {
	Brokers                []string      `toml:"brokers"`
	Username               string        `toml:"username"`
	Password               string        `toml:"password"`
	ConsumerQueueCapacity  int           `toml:"consumerQueueCapacity"`
	ConsumerCommitInterval time.Duration `toml:"consumerCommitInterval"`
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
		cfg.Embedding.Type = embedimpl.DashScope
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

func (c *Config) StorageBucket() string {
	switch c.Storage.Type {
	case storageimpl.Minio:
		return c.Storage.Minio.Bucket
	default:
		return ""
	}
}

func (c *Config) ObjectStorageConfig() (*storage.Config, error) {
	switch c.Storage.Type {
	case storageimpl.Minio:
		presignExpiry := 15 * time.Minute
		if c.Storage.Minio.PresignExpiry != 0 {
			presignExpiry = c.Storage.Minio.PresignExpiry
		}

		return &storage.Config{
			Endpoint:      c.Storage.Minio.Endpoint,
			Region:        c.Storage.Minio.Region,
			Bucket:        c.Storage.Minio.Bucket,
			AccessKey:     c.Storage.Minio.AccessKey,
			SecretKey:     c.Storage.Minio.SecretKey,
			Secure:        c.Storage.Minio.Secure,
			PresignExpiry: presignExpiry,
		}, nil
	default:
		return nil, fmt.Errorf("storage type %q is not supported", c.Storage.Type)
	}
}

type ChunkingConfig struct {
	Size        int `toml:"size"`
	OverlapSize int `toml:"overlapSize"`
}
