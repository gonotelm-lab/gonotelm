package chat

import (
	"strings"

	deepseekext "github.com/cloudwego/eino-ext/components/model/deepseek"
	openaiext "github.com/cloudwego/eino-ext/components/model/openai"
	qwenext "github.com/cloudwego/eino-ext/components/model/qwen"
	einomodel "github.com/cloudwego/eino/components/model"
)

// BuildThinkingOptions builds per-request model options for thinking behavior.
func BuildThinkingOptions(
	providerType Type,
	enableThinking bool,
) []einomodel.Option {
	switch providerType {
	case Openai:
		if !enableThinking {
			return nil
		}
		return []einomodel.Option{openaiext.WithReasoningEffort(openaiext.ReasoningEffortLevelHigh)}
	case Qwen:
		return []einomodel.Option{qwenext.WithEnableThinking(enableThinking)}
	case DeepSeek:
		thinkingType := "disabled"
		if enableThinking {
			thinkingType = "enabled"
		}
		return []einomodel.Option{
			deepseekext.WithExtraFields(map[string]interface{}{
				"thinking": map[string]string{
					"type": thinkingType,
				},
			}),
		}
	default:
		return nil
	}
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
