package shared

import (
	_ "embed"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	embedding "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	text2audio "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	sandboximpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/sandbox"
	storageimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"
)

type LoggingConfig struct {
	Level string `toml:"level"`
}

func (c *LoggingConfig) ApplyDefaults() {
	if c.Level == "" {
		c.Level = "debug"
	}
}

type FlowConfig struct {
	Addr        string        `toml:"addr"`
	Namespace   string        `toml:"namespace"`
	MaxRetry    int           `toml:"maxRetry"`
	DialTimeout time.Duration `toml:"dialTimeout"`
}

func (c *FlowConfig) ApplyDefaults() {
	if c.MaxRetry <= 0 {
		c.MaxRetry = 3
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
}

type DatabaseConfig struct {
	Type     string `toml:"type"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DBName   string `toml:"dbName"`
}

func (d *DatabaseConfig) ToSQLConfig() *sql.Config {
	return &sql.Config{
		Host:     d.Host,
		Port:     d.Port,
		User:     d.User,
		Password: d.Password,
		DBName:   d.DBName,
	}
}

var (
	//go:embed modelprice/deepseek.chat
	defaultDeepSeekPricingScript string

	//go:embed modelprice/dashscope.embedding
	defaultDashScopeEmbeddingPricingScript string
)

type ProviderBillingConfig struct {
	DeepSeekScript           string
	EmbeddingDashScopeScript string
}

func (c *ProviderBillingConfig) Init() {
	if c.DeepSeekScript == "" {
		c.DeepSeekScript = defaultDeepSeekPricingScript
	}
	if c.EmbeddingDashScopeScript == "" {
		c.EmbeddingDashScopeScript = defaultDashScopeEmbeddingPricingScript
	}
}

// InfraConfig 为 notelm / worker 共用的基础设施配置。
type InfraConfig struct {
	Database        DatabaseConfig                `toml:"database"`
	VectorDB        vectordb.Config               `toml:"vectorDb"`
	Storage         storageimpl.StorageTypeConfig `toml:"storage"`
	Provider        llmchat.ProviderConfig        `toml:"provider"`
	ProviderBilling ProviderBillingConfig         `toml:"providerBilling"`
	Embedding       embedding.EmbeddingConfig     `toml:"embedding"`
	Text2Image      text2image.Text2ImageConfig   `toml:"text2image"`
	Text2Audio      text2audio.Text2AudioConfig   `toml:"text2audio"`
	Sandbox         sandboximpl.ProviderConfig    `toml:"sandbox"`
	DatabaseOlap    DatabaseConfig                `toml:"databaseOlap"`

	Redis    cache.RedisCacheConfig `toml:"redis"`
	MsgQueue mqimpl.Config          `toml:"msgQueue"`
}

func (c *InfraConfig) Init() {
	if c.Storage.Type == "" {
		c.Storage.Type = storageimpl.Minio
	}
	if c.Embedding.Type == "" {
		c.Embedding.Type = embedding.EmbeddingDashScope
	}
	if c.Embedding.BatchSize <= 0 {
		c.Embedding.BatchSize = 10
	}
	if c.Embedding.MaxConcurrency <= 0 {
		c.Embedding.MaxConcurrency = 4
	}

	c.ProviderBilling.Init()
}

func (c *InfraConfig) SQLConfig() *sql.Config {
	return c.Database.ToSQLConfig()
}

func IsDevEnv(deployEnv string) bool {
	return deployEnv == "dev"
}
