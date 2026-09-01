package chat

import (
	"net/http"
	"slices"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/model/agnes"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
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

type ModalityType string

const (
	ModalityText  ModalityType = "text"
	ModalityImage ModalityType = "image"
)

type Modality struct {
	Input  []ModalityType `toml:"input"`
	Output []ModalityType `toml:"output"`
}

func (m *Modality) SupportImageInput() bool {
	return slices.Contains(m.Input, ModalityImage)
}

type Model struct {
	Name       string   `toml:"name"`
	Modalities Modality `toml:"modalities"`
}

type DeepSeekChatConfig struct {
	ApiKey           string           `toml:"apiKey"`
	Timeout          time.Duration    `toml:"timeout"`
	BaseURL          string           `toml:"baseUrl"`
	Path             string           `toml:"path"`
	Temperature      *float32         `toml:"temperature"`
	TopP             *float32         `toml:"topP"`
	PresencePenalty  *float32         `toml:"presencePenalty"`
	FrequencyPenalty *float32         `toml:"frequencyPenalty"`
	LogProbs         bool             `toml:"logProbs"`
	TopLogProbs      int              `toml:"topLogProbs"`
	MaxConcurrency   int              `toml:"maxConcurrency"`
	ThinkingEnabled  bool             `toml:"thinkingEnabled"`
	DefaultModel     string           `toml:"defaultModel"`
	Models           map[string]Model `toml:"models"`
}

func (c *DeepSeekChatConfig) ToEino() *deepseek.ChatModelConfig {
	dc := &deepseek.ChatModelConfig{
		APIKey:      c.ApiKey,
		Timeout:     c.Timeout,
		HTTPClient:  newHttpClient(c.Timeout),
		BaseURL:     c.BaseURL,
		Path:        c.Path,
		LogProbs:    c.LogProbs,
		TopLogProbs: c.TopLogProbs,
		Model:       c.DefaultModel,
	}
	if c.Temperature != nil {
		dc.Temperature = *c.Temperature
	}
	if c.TopP != nil {
		dc.TopP = *c.TopP
	}
	if c.PresencePenalty != nil {
		dc.PresencePenalty = *c.PresencePenalty
	}
	if c.FrequencyPenalty != nil {
		dc.FrequencyPenalty = *c.FrequencyPenalty
	}
	if c.ThinkingEnabled {
		dc.ThinkingConfig = &deepseek.ThinkingConfig{Type: "enabled"}
	} else {
		dc.ThinkingConfig = &deepseek.ThinkingConfig{Type: "disabled"}
	}

	return dc
}

func (c *DeepSeekChatConfig) ToOpenaiEino() *openai.ChatModelConfig {
	cc := &openai.ChatModelConfig{
		APIKey:           c.ApiKey,
		Timeout:          c.Timeout,
		HTTPClient:       newHttpClient(c.Timeout),
		BaseURL:          c.BaseURL,
		Model:            c.DefaultModel,
		Temperature:      c.Temperature,
		TopP:             c.TopP,
		FrequencyPenalty: c.FrequencyPenalty,
	}
	if c.ThinkingEnabled {
		cc.ExtraFields = map[string]any{
			"thinking": map[string]any{
				"type": "enabled",
			},
		}
	} else {
		cc.ExtraFields = map[string]any{
			"thinking": map[string]any{
				"type": "disabled",
			},
		}
	}

	return cc
}

type OpenAIChatConfig struct {
	ApiKey           string           `toml:"apiKey"`
	Timeout          time.Duration    `toml:"timeout"`
	BaseUrl          string           `toml:"baseUrl"`
	Model            string           `toml:"model"`
	Temperature      *float32         `toml:"temperature"`
	TopP             *float32         `toml:"topP"`
	PresencePenalty  *float32         `toml:"presencePenalty"`
	Seed             *int             `toml:"seed"`
	FrequencyPenalty *float32         `toml:"frequencyPenalty"`
	ReasoningEffort  string           `toml:"reasoningEffort"` // low, medium, high
	MaxConcurrency   int              `toml:"maxConcurrency"`
	Models           map[string]Model `toml:"models"`
}

func (c *OpenAIChatConfig) ToEino() *openai.ChatModelConfig {
	return &openai.ChatModelConfig{
		APIKey:           c.ApiKey,
		Timeout:          c.Timeout,
		HTTPClient:       httpclient.NewBuilder(nil).WithTimeout(c.Timeout).Build(),
		BaseURL:          c.BaseUrl,
		Model:            c.Model,
		Temperature:      c.Temperature,
		TopP:             c.TopP,
		PresencePenalty:  c.PresencePenalty,
		FrequencyPenalty: c.FrequencyPenalty,
		ReasoningEffort:  openai.ReasoningEffortLevel(c.ReasoningEffort),
	}
}

type QwenChatConfig struct {
	ApiKey           string           `toml:"apiKey"`
	Timeout          time.Duration    `toml:"timeout"`
	BaseUrl          string           `toml:"baseUrl"`
	Model            string           `toml:"model"`
	Temperature      *float32         `toml:"temperature"`
	TopP             *float32         `toml:"topP"`
	PresencePenalty  *float32         `toml:"presencePenalty"`
	Seed             *int             `toml:"seed"`
	FrequencyPenalty *float32         `toml:"frequencyPenalty"`
	EnableThinking   *bool            `toml:"enableThinking"`
	MaxConcurrency   int              `toml:"maxConcurrency"`
	Models           map[string]Model `toml:"models"`
}

func (c *QwenChatConfig) ToEino() *qwen.ChatModelConfig {
	return &qwen.ChatModelConfig{
		APIKey:           c.ApiKey,
		Timeout:          c.Timeout,
		HTTPClient:       newHttpClient(c.Timeout),
		BaseURL:          c.BaseUrl,
		Model:            c.Model,
		Temperature:      c.Temperature,
		TopP:             c.TopP,
		PresencePenalty:  c.PresencePenalty,
		Seed:             c.Seed,
		FrequencyPenalty: c.FrequencyPenalty,
		EnableThinking:   c.EnableThinking,
	}
}

type AgnesChatConfig struct {
	ApiKey           string           `toml:"apiKey"`
	Timeout          time.Duration    `toml:"timeout"`
	BaseUrl          string           `toml:"baseUrl"`
	DefaultModel     string           `toml:"defaultModel"`
	Temperature      *float32         `toml:"temperature"`
	TopP             *float32         `toml:"topP"`
	PresencePenalty  *float32         `toml:"presencePenalty"`
	Seed             *int             `toml:"seed"`
	FrequencyPenalty *float32         `toml:"frequencyPenalty"`
	MaxConcurrency   int              `toml:"maxConcurrency"`
	Models           map[string]Model `toml:"models"`
}

func (c *AgnesChatConfig) ToEino() *agnes.ChatModelConfig {
	return &agnes.ChatModelConfig{
		APIKey:           c.ApiKey,
		Timeout:          c.Timeout,
		HTTPClient:       newHttpClient(c.Timeout),
		BaseURL:          c.BaseUrl,
		Model:            c.DefaultModel,
		Temperature:      c.Temperature,
		TopP:             c.TopP,
		PresencePenalty:  c.PresencePenalty,
		FrequencyPenalty: c.FrequencyPenalty,
		Seed:             c.Seed,
	}
}

func newHttpClient(timeout time.Duration) *http.Client {
	builder := httpclient.NewBuilder(nil)
	if timeout > 0 {
		builder = builder.WithTimeout(timeout)
	}

	return builder.Build()
}
