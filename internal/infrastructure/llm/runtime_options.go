package llm

import (
	"strings"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/model/agnes"
)

var responseFormatJsonObject = map[string]any{
	"response_format": map[string]string{
		"type": "json_object",
	},
}

func WithThinking(
	providerType Provider,
	enableThinking bool,
) einomodel.Option {
	switch providerType {
	case ProviderOpenAI:
		if !enableThinking {
			return einomodel.Option{}
		}
		return openai.WithReasoningEffort(openai.ReasoningEffortLevelHigh)
	case ProviderQwen:
		return qwen.WithEnableThinking(enableThinking)
	case ProviderDeepSeek:
		thinkingType := "disabled"
		if enableThinking {
			thinkingType = "enabled"
		}
		return deepseek.WithExtraFields(map[string]any{
			"thinking": map[string]string{
				"type": thinkingType,
			},
		})
	case ProviderAgnes:
		return agnes.WithExtraFields(map[string]any{
			"chat_template_kwargs": map[string]any{
				"enable_thinking": enableThinking,
			},
		})
	default:
		return einomodel.Option{}
	}
}

func WithModel(model string) einomodel.Option {
	if model != "" {
		return einomodel.WithModel(model)
	}

	return einomodel.Option{}
}

func WithResponseJsonObject(providerType Provider) einomodel.Option {
	switch providerType {
	case ProviderQwen, ProviderDeepSeek, ProviderOpenAI:
		return qwen.WithExtraFields(responseFormatJsonObject)
	default:
		return einomodel.Option{}
	}
}

func BuildLLMOptions(opts ...einomodel.Option) []einomodel.Option {
	return opts
}

func normalizeOpenAIReasoningEffort(
	reasoningEffort string,
) openai.ReasoningEffortLevel {
	switch strings.ToLower(strings.TrimSpace(reasoningEffort)) {
	case string(openai.ReasoningEffortLevelLow):
		return openai.ReasoningEffortLevelLow
	case string(openai.ReasoningEffortLevelHigh):
		return openai.ReasoningEffortLevelHigh
	case string(openai.ReasoningEffortLevelMedium):
		fallthrough
	default:
		return openai.ReasoningEffortLevelMedium
	}
}
