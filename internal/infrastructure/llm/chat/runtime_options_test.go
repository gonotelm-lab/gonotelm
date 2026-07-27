package chat

import (
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
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
		llm.ProviderDeepSeek,
		false,
		[]einomodel.Option{WithThinking(llm.ProviderDeepSeek, false)},
	)
	require.NotEmpty(t, opts)

	callOpts := einomodel.GetImplSpecificOptions(&callOptions{}, opts...)
	require.NotNil(t, callOpts.EnableThinking)
	assert.False(t, *callOpts.EnableThinking)

	fields := deepSeekFieldsForTest(false, false)
	assert.Equal(t, map[string]string{"type": "disabled"}, fields["thinking"])
	assert.NotContains(t, fields, "stream_options")
}

func TestApplyProviderCallOptions_DeepSeekStreamMergesThinkingAndStreamOptions(t *testing.T) {
	fields := deepSeekFieldsForTest(true, false)
	assert.Equal(t, map[string]string{"type": "disabled"}, fields["thinking"])
	assert.Contains(t, fields, "stream_options")

	fieldsEnabled := deepSeekFieldsForTest(true, true)
	assert.Equal(t, map[string]string{"type": "enabled"}, fieldsEnabled["thinking"])
	assert.Contains(t, fieldsEnabled, "stream_options")
}

func deepSeekFieldsForTest(streaming bool, enableThinking bool) map[string]any {
	fields := map[string]any{}
	if streaming {
		fields = mergeExtraFields(fields, streamOptionsIncludeUsage)
	}
	thinkingType := "disabled"
	if enableThinking {
		thinkingType = "enabled"
	}
	fields["thinking"] = map[string]string{"type": thinkingType}
	return fields
}
