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

// callOptions carries gateway-level request options. wrappedChatModel translates
// them into provider-specific options for both Generate and Stream.
type callOptions struct {
	EnableThinking *bool
}

func WithThinking(
	providerType llm.Provider,
	enableThinking bool,
) einomodel.Option {
	_ = providerType // provider is resolved by wrappedChatModel
	enabled := enableThinking
	return einomodel.WrapImplSpecificOptFn(func(o *callOptions) {
		o.EnableThinking = &enabled
	})
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

// applyProviderCallOptions translates callOptions into provider-native options.
// streaming controls whether stream_options is merged (DeepSeek/Qwen ExtraFields replace, not merge).
func applyProviderCallOptions(
	provider llm.Provider,
	streaming bool,
	opts []einomodel.Option,
) []einomodel.Option {
	callOpts := einomodel.GetImplSpecificOptions(&callOptions{}, opts...)

	switch provider {
	case llm.ProviderDeepSeek:
		fields := map[string]any{}
		if streaming {
			fields = mergeExtraFields(fields, streamOptionsIncludeUsage)
		}
		if callOpts.EnableThinking != nil {
			thinkingType := "disabled"
			if *callOpts.EnableThinking {
				thinkingType = "enabled"
			}
			fields["thinking"] = map[string]string{"type": thinkingType}
		}
		if len(fields) > 0 {
			// Append last so this map wins over any earlier deepseek.WithExtraFields.
			opts = append(opts, deepseek.WithExtraFields(fields))
		}
	case llm.ProviderQwen:
		if callOpts.EnableThinking != nil {
			opts = append(opts, qwen.WithEnableThinking(*callOpts.EnableThinking))
		}
		if streaming {
			opts = append(opts, qwen.WithExtraFields(cloneExtraFields(streamOptionsIncludeUsage)))
		}
	case llm.ProviderAgnes:
		if callOpts.EnableThinking != nil {
			opts = append(opts, agnes.WithExtraFields(map[string]any{
				"chat_template_kwargs": map[string]any{
					"enable_thinking": *callOpts.EnableThinking,
				},
			}))
		}
	case llm.ProviderOpenAI:
		if callOpts.EnableThinking != nil && *callOpts.EnableThinking {
			opts = append(opts, openai.WithReasoningEffort(openai.ReasoningEffortLevelHigh))
		}
	}

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
