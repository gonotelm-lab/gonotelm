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

func (c *LoggingConfig) Init() {
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

func (c *FlowConfig) Init() {
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

	//go:embed modelprice/qwen.chat
	defaultQwenChatPricingScript string

	//go:embed modelprice/qwen.embedding
	defaultQwenEmbeddingPricingScript string

	//go:embed modelprice/qwen.text2image
	defaultQwenText2ImagePricingScript string

	//go:embed modelprice/qwen.text2audio
	defaultQwenText2AudioPricingScript string
)

type ProviderBillingConfig struct {
	DeepSeekScript          string
	ChatQwenScript          string
	EmbeddingQwenScript     string
	Text2ImageQwenScript    string
	Text2AudioQwenScript    string
	Text2AudioMiniMaxScript string
}

func (c *ProviderBillingConfig) Init() {
	if c.DeepSeekScript == "" {
		c.DeepSeekScript = defaultDeepSeekPricingScript
	}
	if c.ChatQwenScript == "" {
		c.ChatQwenScript = defaultQwenChatPricingScript
	}
	if c.EmbeddingQwenScript == "" {
		c.EmbeddingQwenScript = defaultQwenEmbeddingPricingScript
	}
	if c.Text2ImageQwenScript == "" {
		c.Text2ImageQwenScript = defaultQwenText2ImagePricingScript
	}
	if c.Text2AudioQwenScript == "" {
		c.Text2AudioQwenScript = defaultQwenText2AudioPricingScript
	}
}

// InfraConfig 共用的基础设施配置。
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
	Redis           cache.RedisCacheConfig        `toml:"redis"`
	MessageQueue    mqimpl.Config                 `toml:"messageQueue"`
}

func (c *InfraConfig) InitInfra() {
	if c.Storage.Type == "" {
		c.Storage.Type = storageimpl.Minio
	}
	if c.Embedding.Type == "" {
		c.Embedding.Type = embedding.EmbeddingQwen
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
