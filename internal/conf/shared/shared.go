package shared

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/sandbox"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"github.com/BurntSushi/toml"
	"github.com/a8m/envsubst"
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
	Database        DatabaseConfig              `toml:"database"`
	VectorDB        vectordb.Config             `toml:"vectorDb"`
	Storage         storage.StorageTypeConfig   `toml:"storage"`
	Provider        llmchat.ProviderConfig      `toml:"provider"`
	ProviderBilling ProviderBillingConfig       `toml:"providerBilling"`
	LLMRecord       llmchat.RecordConfig        `toml:"llmRecord"`
	Embedding       embedding.EmbeddingConfig   `toml:"embedding"`
	Text2Image      text2image.Text2ImageConfig `toml:"text2image"`
	Text2Audio      text2audio.Text2AudioConfig `toml:"text2audio"`
	Sandbox         sandbox.ProviderConfig      `toml:"sandbox"`
	DatabaseOlap    DatabaseConfig              `toml:"databaseOlap"`
	Redis           cache.RedisCacheConfig      `toml:"redis"`
	MessageQueue    mq.Config                   `toml:"messageQueue"`
}

func (c *InfraConfig) InitInfra() {
	if c.Storage.Type == "" {
		c.Storage.Type = storage.Minio
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
	c.LLMRecord.Init()
}

func (c *InfraConfig) SQLConfig() *sql.Config {
	return c.Database.ToSQLConfig()
}

func IsDevEnv(deployEnv string) bool {
	return deployEnv == "dev"
}

func LoadTOML(path string, cfg any) error {
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
