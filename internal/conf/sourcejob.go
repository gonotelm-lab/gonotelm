package conf

import (
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
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

type ChunkingConfig struct {
	Size        int `toml:"size"`
	OverlapSize int `toml:"overlapSize"`
}

func LoadSourceJobConfig(path string) (*SourceJobConfig, error) {
	cfg := &SourceJobConfig{}
	if err := LoadTOML(path, cfg); err != nil {
		return nil, err
	}

	cfg.init()

	sourceJobGlobal = cfg
	return cfg, nil
}

func (c *SourceJobConfig) init() {
	c.InitInfra()
	c.Logging.Init()

	if c.MessageQueue.Type == "" {
		c.MessageQueue.Type = mq.Kafka
	}

	c.examinateSourceConfig()
}

func SourceJobGlobal() *SourceJobConfig {
	return sourceJobGlobal
}

func (c *SourceJobConfig) examinateSourceConfig() {
	// check source config
	var (
		chatModel chat.Model
		models    map[string]chat.Model
	)
	switch c.Source.ImageUnderstandModelProvider {
	case chat.ProviderDeepSeek:
		models = c.Provider.DeepSeek.Models
	case chat.ProviderOpenAI:
		models = c.Provider.OpenAI.Models
	case chat.ProviderQwen:
		models = c.Provider.Qwen.Models
	case chat.ProviderAgnes:
		models = c.Provider.Agnes.Models
	default:
		panic(fmt.Sprintf("unknown image understand model provider %s", c.Source.ImageUnderstandModelProvider))
	}
	chatModel, ok := models[c.Source.ImageUnderstandModel]
	if !ok {
		panic(fmt.Sprintf("image understand model %s not found", c.Source.ImageUnderstandModel))
	}
	if !chatModel.Modalities.SupportImageInput() {
		panic(fmt.Sprintf("image understand model %s does not support image input", c.Source.ImageUnderstandModel))
	}
}
