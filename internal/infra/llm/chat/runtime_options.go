package chat

import (
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/model/agnes"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	einomodel "github.com/cloudwego/eino/components/model"
)

var responseFormatJsonObject = map[string]any{
	"response_format": map[string]string{
		"type": "json_object",
	},
}

// WithThinking builds per-request model options for thinking behavior.
func WithThinking(
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
		return deepseek.WithExtraFields(map[string]any{
			"thinking": map[string]string{
				"type": thinkingType,
			},
		})
	case Agnes:
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

// 要求模型输出 JSON 对象
//
// 注意使用此Option时 需要在提示词中明确要模型输出JSON 否接在某些provider下接口会报错
func WithResponseJsonObject(providerType Provider) einomodel.Option {
	switch providerType {
	case Qwen, DeepSeek, Openai:
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
