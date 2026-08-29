package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/shopspring/decimal"
)

const (
	scriptedCacheHitKey  = "cache_hit_1m"
	scriptedCacheMissKey = "cache_miss_1m"
	scriptedOutputKey    = "output_1m"
)

type tokenPriceProviderUsageEnv struct {
	InputTokens       int64 `expr:"input_tokens"`
	CachedInputTokens int64 `expr:"cached_input_tokens"`
	CacheMissTokens   int64 `expr:"cache_miss_tokens"`
	OutputTokens      int64 `expr:"output_tokens"`
	TotalTokens       int64 `expr:"total_tokens"`
}

type tokenPriceProviderEnv struct {
	Model string                     `expr:"model"`
	Now   time.Time                  `expr:"now"`
	Usage tokenPriceProviderUsageEnv `expr:"usage"`
}

type ScriptedTokenPriceProvider struct {
	vm  *vm.Program
	now func() time.Time
}

func NewScriptedTokenPriceProvider(script string) (*ScriptedTokenPriceProvider, error) {
	program, err := expr.Compile(script, expr.Env(tokenPriceProviderEnv{}))
	if err != nil {
		return nil, fmt.Errorf("compile scripted token price provider script err: %w", err)
	}

	return &ScriptedTokenPriceProvider{
		vm:  program,
		now: time.Now,
	}, nil
}

func (p *ScriptedTokenPriceProvider) Provide(
	ctx context.Context,
	model string,
	usage text2audio.RecordUsage,
) (TokenPrices, error) {
	now := time.Now()
	if p.now != nil {
		now = p.now()
	}

	envUsage := tokenPriceProviderUsageEnv{}
	if usage.TokenUsage != nil {
		tu := usage.TokenUsage
		envUsage = tokenPriceProviderUsageEnv{
			InputTokens:       tu.InputTokens,
			CachedInputTokens: tu.CachedInputTokens,
			CacheMissTokens:   tu.InputTokens - tu.CachedInputTokens,
			OutputTokens:      tu.OutputTokens,
			TotalTokens:       tu.TotalTokens,
		}
	}

	runOut, err := expr.Run(p.vm, tokenPriceProviderEnv{
		Model: model,
		Now:   now,
		Usage: envUsage,
	})
	if err != nil {
		return TokenPrices{}, fmt.Errorf("scripted token price provider run err: %w", err)
	}

	if errMsg, ok := runOut.(string); ok {
		return TokenPrices{}, fmt.Errorf("scripted token price provider error: %s", errMsg)
	}

	prices, ok := runOut.(map[string]any)
	if !ok {
		return TokenPrices{}, fmt.Errorf("scripted token provider output is not map[string]any but %T", runOut)
	}

	ch, ok := prices[scriptedCacheHitKey].(string)
	if !ok {
		return TokenPrices{}, ErrMissingCacheHitPrice
	}
	cm, ok := prices[scriptedCacheMissKey].(string)
	if !ok {
		return TokenPrices{}, ErrMissingCacheMissPrice
	}
	out, ok := prices[scriptedOutputKey].(string)
	if !ok {
		return TokenPrices{}, ErrMissingOutputPrice
	}

	cacheHit, err := decimal.NewFromString(ch)
	if err != nil {
		return TokenPrices{}, ErrCacheHitNotNumber
	}
	cacheMiss, err := decimal.NewFromString(cm)
	if err != nil {
		return TokenPrices{}, ErrCacheMissNotNumber
	}
	output, err := decimal.NewFromString(out)
	if err != nil {
		return TokenPrices{}, ErrOutputNotNumber
	}

	return TokenPrices{
		CacheHitPrice:  cacheHit,
		CacheMissPrice: cacheMiss,
		OutputPrice:    output,
	}, nil
}
