package chat

import (
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	openaiext "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeExtraFields(t *testing.T) {
	base := map[string]any{
		"stream_options": map[string]bool{"include_usage": true},
	}
	merged := mergeExtraFields(base, map[string]any{
		"thinking": map[string]string{"type": "disabled"},
	})

	assert.Equal(t, map[string]bool{"include_usage": true}, merged["stream_options"])
	assert.Equal(t, map[string]string{"type": "disabled"}, merged["thinking"])
	assert.NotContains(t, base, "thinking", "base map must not be mutated")
}

func TestApplyProviderCallOptions_DeepSeekGenerateHasThinkingOnly(t *testing.T) {
	opts := applyProviderCallOptions(
		ProviderDeepSeek,
		false,
		[]einomodel.Option{WithThinking(ProviderDeepSeek, false)},
	)
	require.NotEmpty(t, opts)

	callOpts := einomodel.GetImplSpecificOptions(&callOptions{}, opts...)
	require.NotNil(t, callOpts.EnableThinking)
	assert.False(t, *callOpts.EnableThinking)

	fields := deepSeekFieldsForTest(false, false, false)
	assert.Equal(t, map[string]string{"type": "disabled"}, fields["thinking"])
	assert.NotContains(t, fields, "stream_options")
	assert.NotContains(t, fields, "response_format")
}

func TestApplyProviderCallOptions_DeepSeekStreamMergesThinkingAndStreamOptions(t *testing.T) {
	fields := deepSeekFieldsForTest(true, false, false)
	assert.Equal(t, map[string]string{"type": "disabled"}, fields["thinking"])
	assert.Contains(t, fields, "stream_options")
	assert.NotContains(t, fields, "response_format")

	fieldsEnabled := deepSeekFieldsForTest(true, true, false)
	assert.Equal(t, map[string]string{"type": "enabled"}, fieldsEnabled["thinking"])
	assert.Contains(t, fieldsEnabled, "stream_options")
	assert.NotContains(t, fieldsEnabled, "response_format")
}

func TestApplyProviderCallOptions_DeepSeekResponseFormatMerged(t *testing.T) {
	fields := deepSeekFieldsForTest(true, true, true)
	assert.Equal(t, map[string]string{"type": "enabled"}, fields["thinking"])
	assert.Contains(t, fields, "stream_options")
	assert.Equal(t, map[string]string{"type": "json_object"}, fields["response_format"])
}

func TestApplyProviderCallOptions_ResponseFormatOptions(t *testing.T) {
	t.Run("deepseek", func(t *testing.T) {
		opts := applyProviderCallOptions(
			ProviderDeepSeek,
			false,
			[]einomodel.Option{WithResponseJsonObject(ProviderDeepSeek)},
		)
		require.NotEmpty(t, opts)
		fields := deepSeekFieldsForTest(false, false, true)
		assert.Equal(t, map[string]string{"type": "json_object"}, fields["response_format"])
	})

	t.Run("openai", func(t *testing.T) {
		opts := applyProviderCallOptions(
			ProviderOpenAI,
			false,
			[]einomodel.Option{WithResponseJsonObject(ProviderOpenAI)},
		)
		require.NotEmpty(t, opts)
	})

	t.Run("agnes_appends_chat_template_kwargs", func(t *testing.T) {
		opts := applyProviderCallOptions(
			ProviderAgnes,
			false,
			[]einomodel.Option{WithResponseJsonObject(ProviderAgnes)},
		)
		require.Len(t, opts, 2, "agnes should append one provider-native option")

		opts = applyProviderCallOptions(
			ProviderAgnes,
			false,
			[]einomodel.Option{},
		)
		assert.Len(t, opts, 0, "agnes should not append when no call options set")
	})

	t.Run("agnes_thinking_and_response_format", func(t *testing.T) {
		opts := applyProviderCallOptions(
			ProviderAgnes,
			false,
			[]einomodel.Option{
				WithThinking(ProviderAgnes, false),
				WithResponseJsonObject(ProviderAgnes),
			},
		)
		require.Len(t, opts, 3, "input 2 + appended 1")
	})
}

func deepSeekFieldsForTest(streaming bool, enableThinking bool, responseFormat bool) map[string]any {
	fields := map[string]any{}
	if streaming {
		fields = mergeExtraFields(fields, streamOptionsIncludeUsage)
	}
	thinkingType := "disabled"
	if enableThinking {
		thinkingType = "enabled"
	}
	fields["thinking"] = map[string]string{"type": thinkingType}
	if responseFormat {
		fields = mergeExtraFields(fields, openaiext.ResponseFormatJSONObject)
	}
	return fields
}
