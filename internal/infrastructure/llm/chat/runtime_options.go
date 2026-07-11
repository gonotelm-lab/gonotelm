package chat

import (
	"strings"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/model/agnes"
	openaiext "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/openai"
)

func WithThinking(
	providerType llm.Provider,
	enableThinking bool,
) einomodel.Option {
	switch providerType {
	case llm.ProviderOpenAI:
		if !enableThinking {
			return einomodel.Option{}
		}
		return openai.WithReasoningEffort(openai.ReasoningEffortLevelHigh)
	case llm.ProviderQwen:
		return qwen.WithEnableThinking(enableThinking)
	case llm.ProviderDeepSeek:
		thinkingType := "disabled"
		if enableThinking {
			thinkingType = "enabled"
		}
		return deepseek.WithExtraFields(map[string]any{
			"thinking": map[string]string{
				"type": thinkingType,
			},
		})
	case llm.ProviderAgnes:
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

func WithResponseJsonObject(providerType llm.Provider) einomodel.Option {
	switch providerType {
	case llm.ProviderQwen, llm.ProviderDeepSeek, llm.ProviderOpenAI:
		return qwen.WithExtraFields(openaiext.ResponseFormatJSONObject)
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
