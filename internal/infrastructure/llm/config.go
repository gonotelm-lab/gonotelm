package llm

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
	ProviderOpenAI   Provider = "openai"
	ProviderDeepSeek Provider = "deepseek"
	ProviderQwen     Provider = "qwen"
	ProviderAgnes    Provider = "agnes"
)

type ProviderConfig struct {
	OpenAI   OpenAIChatConfig   `toml:"openai"`
	DeepSeek DeepSeekChatConfig `toml:"deepseek"`
	Qwen     QwenChatConfig     `toml:"qwen"`
	Agnes    AgnesChatConfig    `toml:"agnes"`
}

type OpenAIChatConfig struct {
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

	MaxConcurrency int `toml:"maxConcurrency"`
}

func (c *OpenAIChatConfig) ToEino() *openai.ChatModelConfig {
	return &openai.ChatModelConfig{
		APIKey:              c.ApiKey,
		Timeout:             c.Timeout,
		BaseURL:             c.BaseUrl,
		Model:               c.Model,
		MaxCompletionTokens: c.MaxTokens,
		Temperature:         c.Temperature,
		TopP:                c.TopP,
		PresencePenalty:     c.PresencePenalty,
		FrequencyPenalty:    c.FrequencyPenalty,
		ReasoningEffort:     openai.ReasoningEffortLevel(c.ReasoningEffort),
	}
}

type QwenChatConfig struct {
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

	MaxConcurrency int `toml:"maxConcurrency"`
}

func (c *QwenChatConfig) ToEino() *qwen.ChatModelConfig {
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

type DeepSeekChatConfig struct {
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

	MaxConcurrency int `toml:"maxConcurrency"`
}

func (c *DeepSeekChatConfig) ToEino() *deepseek.ChatModelConfig {
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

type AgnesChatConfig struct {
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

func (c *AgnesChatConfig) ToEino() *agnes.ChatModelConfig {
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
