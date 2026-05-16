package chat

import (
	"strings"

	deepseekext "github.com/cloudwego/eino-ext/components/model/deepseek"
	openaiext "github.com/cloudwego/eino-ext/components/model/openai"
	qwenext "github.com/cloudwego/eino-ext/components/model/qwen"
	einomodel "github.com/cloudwego/eino/components/model"
)

// BuildThinkingOption builds per-request model options for thinking behavior.
func BuildThinkingOption(
	providerType Type,
	enableThinking bool,
) einomodel.Option {
	switch providerType {
	case Openai:
		if !enableThinking {
			return einomodel.Option{}
		}
		return openaiext.WithReasoningEffort(openaiext.ReasoningEffortLevelHigh)
	case Qwen:
		return qwenext.WithEnableThinking(enableThinking)
	case DeepSeek:
		thinkingType := "disabled"
		if enableThinking {
			thinkingType = "enabled"
		}
		return deepseekext.WithExtraFields(map[string]interface{}{
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
) openaiext.ReasoningEffortLevel {
	switch strings.ToLower(strings.TrimSpace(reasoningEffort)) {
	case string(openaiext.ReasoningEffortLevelLow):
		return openaiext.ReasoningEffortLevelLow
	case string(openaiext.ReasoningEffortLevelHigh):
		return openaiext.ReasoningEffortLevelHigh
	case string(openaiext.ReasoningEffortLevelMedium):
		fallthrough
	default:
		return openaiext.ReasoningEffortLevelMedium
	}
}
