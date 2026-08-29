package billing

import (
	"context"
	_ "embed"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/dashscope.text2audio
var testDashscopeText2AudioScript string

func ptrInt64(v int64) *int64 { return &v }

func TestNewScriptedCharacterPriceProvider_CompilesDashScopeScript(t *testing.T) {
	_, err := NewScriptedCharacterPriceProvider(testDashscopeText2AudioScript)
	require.NoError(t, err)
}

func TestScriptedCharacterPriceProvider_DashScopeModels(t *testing.T) {
	p, err := NewScriptedCharacterPriceProvider(testDashscopeText2AudioScript)
	require.NoError(t, err)

	cases := []struct {
		model string
		want  string
	}{
		{"qwen-audio-3.0-tts-flash", "1"},
		{"cosyvoice-v3.5-plus", "1.5"},
		{"qwen3-tts-instruct-flash", "0.8"},
		{"qwen3-tts-vd-2026-01-26", "0.8"},
		{"qwen3-tts-vc-2026-01-22", "0.8"},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			got, err := p.Provide(context.Background(), tc.model, text2audio.RecordUsage{
				Characters: ptrInt64(1),
			})
			require.NoError(t, err)
			want := decimal.RequireFromString(tc.want)
			assert.True(t, want.Equal(got.CharacterPrice), "got=%s want=%s", got.CharacterPrice, want)
		})
	}
}

func TestScriptedCharacterPriceProvider_UnknownModel(t *testing.T) {
	p, err := NewScriptedCharacterPriceProvider(testDashscopeText2AudioScript)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "unknown-model", text2audio.RecordUsage{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model not supported to provide price")
}

func TestCharacterBasedMeter_CalculateBy10kCharacters(t *testing.T) {
	meter := NewCharacterBasedMeter()
	p, err := NewScriptedCharacterPriceProvider(testDashscopeText2AudioScript)
	require.NoError(t, err)
	meter.SetProvider(text2audio.Text2AudioDashScope, p)

	// 10000 chars * ¥1 / 万字符 = ¥1
	total, details, err := meter.Calculate(
		context.Background(),
		text2audio.Text2AudioDashScope,
		"qwen-audio-3.0-tts-flash",
		text2audio.RecordUsage{Characters: ptrInt64(10_000)},
	)
	require.NoError(t, err)
	require.NotNil(t, total)
	assert.True(t, decimal.RequireFromString("1").Equal(*total))
	assert.True(t, decimal.RequireFromString("1").Equal(details[CharacterPriceKey]))
}

func TestCharacterBasedMeter_MissingCharacters(t *testing.T) {
	meter := NewCharacterBasedMeter()
	p, err := NewScriptedCharacterPriceProvider(`{character_10k: "1"}`)
	require.NoError(t, err)
	meter.SetProvider(text2audio.Text2AudioDashScope, p)

	_, _, err = meter.Calculate(
		context.Background(),
		text2audio.Text2AudioDashScope,
		"any",
		text2audio.RecordUsage{},
	)
	require.ErrorIs(t, err, ErrMissingCharacters)
}

func TestTokenBasedMeter_Calculate(t *testing.T) {
	meter := NewTokenBasedMeter()
	p, err := NewScriptedTokenPriceProvider(`{
		cache_hit_1m: "1",
		cache_miss_1m: "2",
		output_1m: "4"
	}`)
	require.NoError(t, err)
	meter.SetProvider(text2audio.Text2AudioMimo, p)

	total, details, err := meter.Calculate(
		context.Background(),
		text2audio.Text2AudioMimo,
		"any-model",
		text2audio.RecordUsage{
			TokenUsage: &text2audio.RecordTokenUsage{
				InputTokens:       1_000_000,
				CachedInputTokens: 250_000,
				OutputTokens:      500_000,
				TotalTokens:       1_500_000,
			},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, total)
	assert.True(t, decimal.RequireFromString("3.75").Equal(*total))
	assert.True(t, decimal.RequireFromString("0.25").Equal(details[CacheHitPriceKey]))
	assert.True(t, decimal.RequireFromString("1.5").Equal(details[CacheMissPriceKey]))
	assert.True(t, decimal.RequireFromString("2").Equal(details[OutputPriceKey]))
}

func TestStandardMeter_RoutesCharacterProviders(t *testing.T) {
	meter, err := NewStandardMeter(StandardMeterConfig{
		DashScopeScript: testDashscopeText2AudioScript,
		MiniMaxScript:   `{character_10k: "2"}`,
	})
	require.NoError(t, err)

	total, _, err := meter.Calculate(
		context.Background(),
		text2audio.Text2AudioDashScope,
		"cosyvoice-v3.5-plus",
		text2audio.RecordUsage{Characters: ptrInt64(10_000)},
	)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1.5").Equal(*total))

	total, _, err = meter.Calculate(
		context.Background(),
		text2audio.Text2AudioMiniMax,
		"minimax-tts",
		text2audio.RecordUsage{Characters: ptrInt64(5_000)},
	)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1").Equal(*total))
}

func TestStandardMeter_MimoNotBilled(t *testing.T) {
	meter, err := NewStandardMeter(StandardMeterConfig{
		DashScopeScript: testDashscopeText2AudioScript,
	})
	require.NoError(t, err)

	total, details, err := meter.Calculate(
		context.Background(),
		text2audio.Text2AudioMimo,
		"mimo-tts",
		text2audio.RecordUsage{
			TokenUsage: &text2audio.RecordTokenUsage{InputTokens: 1_000_000},
		},
	)
	require.NoError(t, err)
	assert.Nil(t, total)
	assert.Nil(t, details)
}

func TestScriptedCharacterPriceProvider_MissingKey(t *testing.T) {
	p, err := NewScriptedCharacterPriceProvider(`{image: "1"}`)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "any", text2audio.RecordUsage{})
	require.ErrorIs(t, err, ErrMissingCharacterPrice)
}
