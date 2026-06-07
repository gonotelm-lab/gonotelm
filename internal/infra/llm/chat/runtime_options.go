package chat

import (
	"strings"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	einomodel "github.com/cloudwego/eino/components/model"
)

// BuildThinkingOption builds per-request model options for thinking behavior.
func BuildThinkingOption(
	providerType Provider,
	enableThinking bool,
) einomodel.Option {
	switch providerType {
	case Openai:
		if !enableThinking {
			return einomodel.Option{}
		}
		return openai.WithReasoningEffort(openai.ReasoningEffortLevelHigh)
	case Qwen:
		return qwen.WithEnableThinking(enableThinking)
	case DeepSeek:
		thinkingType := "disabled"
		if enableThinking {
			thinkingType = "enabled"
		}
		return deepseek.WithExtraFields(map[string]interface{}{
			"thinking": map[string]string{
				"type": thinkingType,
			},
		})
	default:
		return einomodel.Option{}
	}
}

func BuildLLMModelOption(model string) einomodel.Option {
	if model != "" {
		return einomodel.WithModel(model)
	}

	return einomodel.Option{}
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
