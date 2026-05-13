package chat

import (
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
)

type Type string

func (t Type) String() string {
	return string(t)
}

const (
	Openai   Type = "openai"
	DeepSeek Type = "deepseek"
	Qwen     Type = "qwen"
)

type ProviderConfig struct {
	Openai   OpenaiConfig   `toml:"openai"`
	DeepSeek DeepSeekConfig `toml:"deepseek"`
	Qwen     QwenConfig     `toml:"qwen"`
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
