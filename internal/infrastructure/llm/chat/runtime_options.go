package chat

import (
	"context"
	"maps"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/model/agnes"
	openaiext "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/openai"
)

var streamOptionsIncludeUsage = openaiext.StreamOptionsIncludeUsage

func cloneExtraFields(base map[string]any) map[string]any {
	if len(base) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(base))
	maps.Copy(out, base)
	return out
}

// mergeExtraFields returns a new map with overrides applied on top of base.
// eino-ext WithExtraFields replaces the whole map, so callers must merge explicitly.
func mergeExtraFields(base map[string]any, overrides map[string]any) map[string]any {
	out := cloneExtraFields(base)
	maps.Copy(out, overrides)
	return out
}

// callOptions carries gateway-level request options. wrappedChatModel translates
// them into provider-specific options for both Generate and Stream.
type callOptions struct {
	EnableThinking           *bool
	ResponseFormatJSONObject bool
}

func WithThinking(
	providerType Provider,
	enableThinking bool,
) einomodel.Option {
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

func WithResponseJsonObject(providerType Provider) einomodel.Option {
	return einomodel.WrapImplSpecificOptFn(func(o *callOptions) {
		o.ResponseFormatJSONObject = true
	})
}

func BuildLLMOptions(opts ...einomodel.Option) []einomodel.Option {
	return opts
}

// applyProviderCallOptions translates callOptions into provider-native options.
// streaming controls whether stream_options is merged (DeepSeek/Qwen ExtraFields replace, not merge).
func applyProviderCallOptions(
	provider Provider,
	streaming bool,
	opts []einomodel.Option,
) ([]einomodel.Option, *callOptions) {
	callOpts := einomodel.GetImplSpecificOptions(&callOptions{}, opts...)

	switch provider {
	case ProviderDeepSeek:
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
		if callOpts.ResponseFormatJSONObject {
			fields = mergeExtraFields(fields, openaiext.ResponseFormatJSONObject)
		}
		if len(fields) > 0 {
			// Append last so this map wins over any earlier deepseek.WithExtraFields.
			opts = append(opts, deepseek.WithExtraFields(fields))
		}
	case ProviderQwen:
		fields := map[string]any{}
		if streaming {
			fields = mergeExtraFields(fields, streamOptionsIncludeUsage)
		}
		if callOpts.EnableThinking != nil {
			opts = append(opts, qwen.WithEnableThinking(*callOpts.EnableThinking))
		}
		if callOpts.ResponseFormatJSONObject {
			fields = mergeExtraFields(fields, openaiext.ResponseFormatJSONObject)
		}
		if len(fields) > 0 {
			opts = append(opts, qwen.WithExtraFields(fields))
		}
	case ProviderAgnes:
		chatTemplateKwargs := map[string]any{}
		if callOpts.EnableThinking != nil {
			chatTemplateKwargs["enable_thinking"] = *callOpts.EnableThinking
		}
		if callOpts.ResponseFormatJSONObject {
			chatTemplateKwargs["response_format"] = map[string]string{"type": "json_object"}
		}
		if len(chatTemplateKwargs) > 0 {
			opts = append(opts, agnes.WithExtraFields(map[string]any{
				"chat_template_kwargs": chatTemplateKwargs,
			}))
		}
	case ProviderOpenAI:
		if callOpts.EnableThinking != nil && *callOpts.EnableThinking {
			opts = append(opts, openai.WithReasoningEffort(openai.ReasoningEffortLevelHigh))
		}
		if callOpts.ResponseFormatJSONObject {
			opts = append(opts, openai.WithExtraFields(openaiext.ResponseFormatJSONObject))
		}
	}

	return opts, callOpts
}

func withCallOptions(ctx context.Context, callOpts *callOptions) context.Context {
	if callOpts != nil && callOpts.EnableThinking != nil {
		ctx = withThinking(ctx, *callOpts.EnableThinking)
	}

	return ctx
}
