package billing

import (
	"context"
	_ "embed"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/qwen.embedding
	testQwenEmbeddingScript string
)

func TestNewScriptedPriceProvider_CompilesModelPriceScripts(t *testing.T) {
	_, err := NewScriptedPriceProvider(testQwenEmbeddingScript)
	require.NoError(t, err)
}

func TestNewScriptedPriceProvider_InvalidScript(t *testing.T) {
	_, err := NewScriptedPriceProvider("{{{")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compile")
}

func TestScriptedPriceProvider_QwenEmbedding(t *testing.T) {
	p, err := NewScriptedPriceProvider(testQwenEmbeddingScript)
	require.NoError(t, err)

	cases := []struct {
		name  string
		model string
		want  string
	}{
		{"qwen3.7-text-embedding", "qwen3.7-text-embedding", "0.5"},
		{"text-embedding-v4", "text-embedding-v4", "0.5"},
		{"text-embedding-v2", "text-embedding-v2", "0.7"},
		{"text-embedding-v1", "text-embedding-v1", "0.7"},
		{"unknown_falls_back_default", "unknown-model", "0.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Provide(context.Background(), tc.model, embedding.RecordTokenUsage{
				PromptTokens: 1000,
			})
			require.NoError(t, err)
			want := decimal.RequireFromString(tc.want)
			assert.True(t, want.Equal(got.PromptPrice), "prompt got=%s want=%s", got.PromptPrice, want)
		})
	}
}

func TestScriptedPriceProvider_UsageAvailable(t *testing.T) {
	p, err := NewScriptedPriceProvider(`
		usage.prompt_tokens > 1000 ?
			{prompt_1m: "1.0"} :
			{prompt_1m: "0.5"}
	`)
	require.NoError(t, err)

	low, err := p.Provide(context.Background(), "any", embedding.RecordTokenUsage{PromptTokens: 1000})
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("0.5").Equal(low.PromptPrice))

	high, err := p.Provide(context.Background(), "any", embedding.RecordTokenUsage{PromptTokens: 1001})
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1.0").Equal(high.PromptPrice))
}

func TestToPriceProviderUsageEnv(t *testing.T) {
	got := toPriceProviderUsageEnv(embedding.RecordTokenUsage{
		PromptTokens:     100,
		CompletionTokens: 20,
		TotalTokens:      120,
	})
	assert.Equal(t, PriceProviderUsageEnv{
		PromptTokens:     100,
		CompletionTokens: 20,
		TotalTokens:      120,
	}, got)
}

func TestScriptedPriceProvider_ScriptErrorString(t *testing.T) {
	p, err := NewScriptedPriceProvider(`'model not supported to provide price'`)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "any", embedding.RecordTokenUsage{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model not supported to provide price")
}

func TestScriptedPriceProvider_MissingPromptKey(t *testing.T) {
	p, err := NewScriptedPriceProvider(`{cache_hit_1m: "1"}`)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "any", embedding.RecordTokenUsage{})
	require.ErrorIs(t, err, ErrMissingPromptPrice)
}

func TestScriptedPriceProvider_InvalidPriceNumber(t *testing.T) {
	p, err := NewScriptedPriceProvider(`{prompt_1m: "abc"}`)
	require.NoError(t, err)

	_, err = p.Provide(context.Background(), "any", embedding.RecordTokenUsage{})
	require.ErrorIs(t, err, ErrPromptNotNumber)
}
