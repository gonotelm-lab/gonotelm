package chat

import (
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/model/agnes"
)

type Provider string

func (t Provider) String() string {
	return string(t)
}

const (
	Openai   Provider = "openai"
	DeepSeek Provider = "deepseek"
	Qwen     Provider = "qwen"
	Agnes    Provider = "agnes"
)

type ProviderConfig struct {
	Openai   OpenaiConfig   `toml:"openai"`
	DeepSeek DeepSeekConfig `toml:"deepseek"`
	Qwen     QwenConfig     `toml:"qwen"`
	Agnes    AgnesConfig    `toml:"agnes"`
}

type OpenaiConfig struct {
	ApiKey           string        `toml:"apiKey"`
	Timeout          time.Duration `toml:"timeout"`
	BaseUrl          string        `toml:"baseUrl"`
	Model            string        `toml:"model"`
	MaxTokens        *int          `toml:"maxTokens"`
	Temperature      *float32      `toml:"temperature"`
	TopP             *float32      `toml:"topP"`
	PresencePenalty  *float32      `toml:"presencePenalty"`
	Seed             *int          `toml:"seed"`
	FrequencyPenalty *float32      `toml:"frequencyPenalty"`
	ReasoningEffort  string        `toml:"reasoningEffort"` // low, medium, high

	// Gateway最大并发调用数 通过合理设置此值防止被限流
	MaxConcurrency int `toml:"maxConcurrency"`
}

func (c *OpenaiConfig) ToEino() *openai.ChatModelConfig {
	return &openai.ChatModelConfig{
		APIKey:              c.ApiKey,
		Timeout:             c.Timeout,
		BaseURL:             c.BaseUrl,
		Model:               c.Model,
		MaxCompletionTokens: c.MaxTokens,
		Temperature:         c.Temperature,      // 1.0 by default
		TopP:                c.TopP,             // 1.0 by default
		PresencePenalty:     c.PresencePenalty,  // 0.0 by default
		FrequencyPenalty:    c.FrequencyPenalty, // 0.0 by default
		ReasoningEffort:     openai.ReasoningEffortLevel(c.ReasoningEffort),
	}
}

type QwenConfig struct {
	ApiKey           string        `toml:"apiKey"`
	Timeout          time.Duration `toml:"timeout"`
	BaseUrl          string        `toml:"baseUrl"`
	Model            string        `toml:"model"`
	MaxTokens        *int          `toml:"maxTokens"`
	Temperature      *float32      `toml:"temperature"`
	TopP             *float32      `toml:"topP"`
	PresencePenalty  *float32      `toml:"presencePenalty"`
	Seed             *int          `toml:"seed"`
	FrequencyPenalty *float32      `toml:"frequencyPenalty"`
	EnableThinking   *bool         `toml:"enableThinking"`

	// Gateway最大并发调用数 通过合理设置此值防止被限流
	MaxConcurrency int `toml:"maxConcurrency"`
}

func (c *QwenConfig) ToEino() *qwen.ChatModelConfig {
	return &qwen.ChatModelConfig{
		APIKey:           c.ApiKey,
		Timeout:          c.Timeout,
		BaseURL:          c.BaseUrl,
		Model:            c.Model,
		MaxTokens:        c.MaxTokens,
		Temperature:      c.Temperature,
		TopP:             c.TopP,
		PresencePenalty:  c.PresencePenalty,
		Seed:             c.Seed,
		FrequencyPenalty: c.FrequencyPenalty,
		EnableThinking:   c.EnableThinking,
	}
}

type DeepSeekConfig struct {
	ApiKey           string        `toml:"apiKey"`
	Timeout          time.Duration `toml:"timeout"`
	BaseURL          string        `toml:"baseUrl"`
	Path             string        `toml:"path"`
	Model            string        `toml:"model"`
	MaxTokens        int           `toml:"maxTokens"`
	Temperature      float32       `toml:"temperature"`
	TopP             float32       `toml:"topP"`
	PresencePenalty  float32       `toml:"presencePenalty"`
	FrequencyPenalty float32       `toml:"frequencyPenalty"`
	LogProbs         bool          `toml:"logProbs"`
	TopLogProbs      int           `toml:"topLogProbs"`
	ThinkingEnabled  bool          `toml:"thinkingEnabled"`

	// Gateway最大并发调用数 通过合理设置此值防止被限流
	MaxConcurrency int `toml:"maxConcurrency"`
}

func (c *DeepSeekConfig) ToEino() *deepseek.ChatModelConfig {
	dc := &deepseek.ChatModelConfig{
		APIKey:           c.ApiKey,
		Timeout:          c.Timeout,
		BaseURL:          c.BaseURL,
		Path:             c.Path,
		Model:            c.Model,
		MaxTokens:        c.MaxTokens,
		Temperature:      c.Temperature,
		TopP:             c.TopP,
		PresencePenalty:  c.PresencePenalty,
		FrequencyPenalty: c.FrequencyPenalty,
		LogProbs:         c.LogProbs,
		TopLogProbs:      c.TopLogProbs,
	}
	if c.ThinkingEnabled {
		dc.ThinkingConfig.Type = "enabled"
	}

	return dc
}

type AgnesConfig struct {
	ApiKey           string        `toml:"apiKey"`
	Timeout          time.Duration `toml:"timeout"`
	BaseUrl          string        `toml:"baseUrl"`
	Model            string        `toml:"model"`
	MaxTokens        *int          `toml:"maxTokens"`
	Temperature      *float32      `toml:"temperature"`
	TopP             *float32      `toml:"topP"`
	PresencePenalty  *float32      `toml:"presencePenalty"`
	Seed             *int          `toml:"seed"`
	FrequencyPenalty *float32      `toml:"frequencyPenalty"`
	MaxConcurrency   int           `toml:"maxConcurrency"`
}

func (c *AgnesConfig) ToEino() *agnes.ChatModelConfig {
	return &agnes.ChatModelConfig{
		APIKey:              c.ApiKey,
		Timeout:             c.Timeout,
		BaseURL:             c.BaseUrl,
		Model:               c.Model,
		MaxCompletionTokens: c.MaxTokens,
		Temperature:         c.Temperature,
		TopP:                c.TopP,
		PresencePenalty:     c.PresencePenalty,
		FrequencyPenalty:    c.FrequencyPenalty,
		Seed:                c.Seed,
	}
}
