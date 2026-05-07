package chat

import (
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

type Type string

const (
	Openai   Type = "openai"
	DeepSeek Type = "deepseek"
)

type Config struct {
	Type Type `toml:"type"`

	Openai   OpenaiConfig   `toml:"openai"`
	DeepSeek DeepSeekConfig `toml:"deepseek"`
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
