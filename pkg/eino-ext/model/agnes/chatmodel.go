package agnes

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var (
	_ model.ToolCallingChatModel = (*ChatModel)(nil)
	_ model.ChatModel            = (*ChatModel)(nil)
)

type ChatModelConfig struct {
	APIKey              string        `json:"api_key"`
	Timeout             time.Duration `json:"timeout"`
	HTTPClient          *http.Client  `json:"http_client"`
	BaseURL             string        `json:"base_url"`
	Model               string        `json:"model"`
	MaxTokens           *int          `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int          `json:"max_completion_tokens,omitempty"`
	Temperature         *float32      `json:"temperature,omitempty"`
	TopP                *float32      `json:"top_p,omitempty"`
	Stop                []string      `json:"stop,omitempty"`
	PresencePenalty     *float32      `json:"presence_penalty,omitempty"`
	Seed                *int          `json:"seed,omitempty"`
	FrequencyPenalty    *float32      `json:"frequency_penalty,omitempty"`
}

type ChatModel struct {
	cli *openai.Client
}

func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	var nConf *openai.Config
	if config != nil {
		var httpClient *http.Client
		if config.HTTPClient != nil {
			httpClient = config.HTTPClient
		} else {
			httpClient = &http.Client{Timeout: config.Timeout}
		}

		baseURL := config.BaseURL
		if baseURL == "" {
			baseURL = "https://apihub.agnes-ai.com/v1"
		}

		nConf = &openai.Config{
			BaseURL:             baseURL,
			APIKey:              config.APIKey,
			HTTPClient:          httpClient,
			Model:               config.Model,
			MaxTokens:           config.MaxTokens,
			MaxCompletionTokens: config.MaxCompletionTokens,
			Temperature:         config.Temperature,
			TopP:                config.TopP,
			Stop:                config.Stop,
			PresencePenalty:     config.PresencePenalty,
			Seed:                config.Seed,
			FrequencyPenalty:    config.FrequencyPenalty,
		}
	}
	cli, err := openai.NewClient(ctx, nConf)
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		cli: cli,
	}, nil
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	out, err := cm.cli.Generate(ctx, in, opts...)
	if err != nil {
		return nil, convOrigAPIError(err)
	}
	return out, nil
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	out, err := cm.cli.Stream(ctx, in, opts...)
	if err != nil {
		return nil, convOrigAPIError(err)
	}
	return out, nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cli, err := cm.cli.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &ChatModel{cli: cli}, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindTools(tools)
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	return cm.cli.BindForcedTools(tools)
}

func (cm *ChatModel) GetType() string {
	return "Agnes"
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return cm.cli.IsCallbacksEnabled()
}
