package billing

import (
	"context"
	_ "embed"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/dashscope.chat
	testDashscopeChatScript string

	//go:embed testdata/deepseek.chat
	testDeepseekChatScript string
)

func mustPrices(hit, miss, out string) TokenPrices {
	return TokenPrices{
		CacheHitPrice:  decimal.RequireFromString(hit),
		CacheMissPrice: decimal.RequireFromString(miss),
		OutputPrice:    decimal.RequireFromString(out),
	}
}

func assertPrices(t *testing.T, got, want TokenPrices) {
	t.Helper()
	assert.True(t, want.CacheHitPrice.Equal(got.CacheHitPrice), "cache_hit got=%s want=%s", got.CacheHitPrice, want.CacheHitPrice)
	assert.True(t, want.CacheMissPrice.Equal(got.CacheMissPrice), "cache_miss got=%s want=%s", got.CacheMissPrice, want.CacheMissPrice)
	assert.True(t, want.OutputPrice.Equal(got.OutputPrice), "output got=%s want=%s", got.OutputPrice, want.OutputPrice)
}

func TestNewScriptedPriceProvider_CompilesModelPriceScripts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		script string
	}{
		{"dashscope.chat", testDashscopeChatScript},
		{"deepseek.chat", testDeepseekChatScript},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewScriptedPriceProvider(tc.script)
			require.NoError(t, err)
		})
	}
}

func TestNewScriptedPriceProvider_InvalidScript(t *testing.T) {
	_, err := NewScriptedPriceProvider("{{{")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compile")
}

func TestScriptedPriceProvider_DashScopeChat(t *testing.T) {
	p, err := NewScriptedPriceProvider(testDashscopeChatScript)
	require.NoError(t, err)

	cases := []struct {
		name   string
		model  string
		prompt int
		want   TokenPrices
	}{
		{"qwen3.8-flash", "qwen3.8-flash", 0, mustPrices("0.1", "0.8", "2.7")},
		{"qwen3.8-max", "qwen3.8-max", 0, mustPrices("1.5", "12", "36")},
		{"qwen3.7-max", "qwen3.7-max", 0, mustPrices("1.2", "6", "18")},
		{"qwen3.7-plus_le_256k", "qwen3.7-plus", 256000, mustPrices("0.32", "1.6", "6.4")},
		{"qwen3.7-plus_gt_256k", "qwen3.7-plus", 256001, mustPrices("0.96", "4.8", "19.2")},
		{"qwen3.7-flash_le_32k", "qwen3.7-flash", 32000, mustPrices("0.04", "0.2", "0.8")},
		{"qwen3.7-flash_32k_to_256k", "qwen3.7-flash", 32001, mustPrices("0.12", "0.6", "2.4")},
		{"qwen3.7-flash_eq_256k", "qwen3.7-flash", 256000, mustPrices("0.12", "0.6", "2.4")},
		{"qwen3.7-flash_gt_256k", "qwen3.7-flash", 256001, mustPrices("0.24", "1.2", "4.8")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Provide(context.Background(), tc.model, chat.RecordTokenUsage{
				PromptTokens: tc.prompt,
			})
			require.NoError(t, err)
			assertPrices(t, got, tc.want)
		})
	}
}

func TestScriptedPriceProvider_DashScopeChat_UnsupportedModel(t *testing.T) {
	p, err := NewScriptedPriceProvider(testDashscopeChatScript)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "unknown-model", chat.RecordTokenUsage{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model not supported to provide price")
}

func TestScriptedPriceProvider_DeepSeekChat(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	p, err := NewScriptedPriceProvider(testDeepseekChatScript)
	require.NoError(t, err)

	cases := []struct {
		name  string
		model string
		now   time.Time
		want  TokenPrices
	}{
		{
			name:  "peak_weekday_morning_pro",
			model: "deepseek-v4-pro",
			now:   time.Date(2026, time.August, 24, 10, 0, 0, 0, shanghai),
			want:  mustPrices("0.30", "9.0", "27.0"),
		},
		{
			name:  "peak_weekday_morning_default",
			model: "deepseek-chat",
			now:   time.Date(2026, time.August, 24, 10, 0, 0, 0, shanghai),
			want:  mustPrices("0.10", "3.0", "9.0"),
		},
		{
			name:  "peak_weekday_afternoon_pro",
			model: "deepseek-v4-pro",
			now:   time.Date(2026, time.August, 26, 15, 0, 0, 0, shanghai),
			want:  mustPrices("0.30", "9.0", "27.0"),
		},
		{
			name:  "offpeak_weekday_lunch_pro",
			model: "deepseek-v4-pro",
			now:   time.Date(2026, time.August, 25, 13, 0, 0, 0, shanghai),
			want:  mustPrices("0.15", "4.5", "13.5"),
		},
		{
			name:  "offpeak_weekday_night_default",
			model: "deepseek-chat",
			now:   time.Date(2026, time.August, 24, 22, 0, 0, 0, shanghai),
			want:  mustPrices("0.05", "1.5", "4.5"),
		},
		{
			name:  "weekend_morning_is_offpeak_pro",
			model: "deepseek-v4-pro",
			now:   time.Date(2026, time.August, 29, 10, 0, 0, 0, shanghai),
			want:  mustPrices("0.15", "4.5", "13.5"),
		},
		{
			name:  "weekday_09_peak_start",
			model: "deepseek-chat",
			now:   time.Date(2026, time.August, 24, 9, 0, 0, 0, shanghai),
			want:  mustPrices("0.10", "3.0", "9.0"),
		},
		{
			name:  "weekday_12_peak_end",
			model: "deepseek-chat",
			now:   time.Date(2026, time.August, 24, 12, 0, 0, 0, shanghai),
			want:  mustPrices("0.05", "1.5", "4.5"),
		},
		{
			name:  "weekday_18_peak_end",
			model: "deepseek-chat",
			now:   time.Date(2026, time.August, 28, 18, 0, 0, 0, shanghai),
			want:  mustPrices("0.05", "1.5", "4.5"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p.now = func() time.Time { return tc.now }
			got, err := p.Provide(context.Background(), tc.model, chat.RecordTokenUsage{})
			require.NoError(t, err)
			assertPrices(t, got, tc.want)
		})
	}
}

func TestScriptedPriceProvider_InvalidPriceNumber(t *testing.T) {
	p, err := NewScriptedPriceProvider(`{
		cache_hit_1m: "abc",
		cache_miss_1m: "1",
		output_1m: "1"
	}`)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "any", chat.RecordTokenUsage{})
	require.ErrorIs(t, err, ErrCacheHitNotNumber)
}

func TestScriptedPriceProvider_MissingKeys(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   error
	}{
		{"missing_cache_hit", `{cache_miss_1m: "1", output_1m: "1"}`, ErrMissingCacheHitPrice},
		{"missing_cache_miss", `{cache_hit_1m: "1", output_1m: "1"}`, ErrMissingCacheMissPrice},
		{"missing_output", `{cache_hit_1m: "1", cache_miss_1m: "1"}`, ErrMissingOutputPrice},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewScriptedPriceProvider(tc.script)
			require.NoError(t, err)
			_, err = p.Provide(context.Background(), "any", chat.RecordTokenUsage{})
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestToPriceProviderUsageEnv(t *testing.T) {
	got := toPriceProviderUsageEnv(chat.RecordTokenUsage{
		PromptTokens:       100,
		PromptCachedTokens: 30,
		CompletionTokens:   20,
	})
	assert.Equal(t, PriceProviderUsageEnv{
		PromptTokens:          100,
		PromptCacheHitTokens:  30,
		PromptCacheMissTokens: 70,
		CompletionTokens:      20,
	}, got)
}
