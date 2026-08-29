package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/shopspring/decimal"
)

// and it should be map[string]any, any is of type string
const (
	scriptedCacheHitKey  = "cache_hit_1m"
	scriptedCacheMissKey = "cache_miss_1m"
	scriptedOutputKey    = "output_1m"
)

type PriceProviderUsageEnv struct {
	// Total prompt input tokens, including cache hit and cache miss
	PromptTokens int `expr:"prompt_tokens"`

	// cached input prompt tokens
	PromptCacheHitTokens int `expr:"prompt_cache_hit_tokens"`

	// cache miss input prompt tokens
	PromptCacheMissTokens int `expr:"prompt_cache_miss_tokens"`

	// output tokens
	CompletionTokens int `expr:"completion_tokens"`
}

func toPriceProviderUsageEnv(usage chat.RecordTokenUsage) PriceProviderUsageEnv {
	return PriceProviderUsageEnv{
		PromptTokens:          usage.PromptTokens,
		PromptCacheHitTokens:  usage.PromptCachedTokens,
		PromptCacheMissTokens: usage.PromptTokens - usage.PromptCachedTokens,
		CompletionTokens:      usage.CompletionTokens,
	}
}

type PriceProviderEnv struct {
	Model string                `expr:"model"` // model name
	Now   time.Time             `expr:"now"`
	Usage PriceProviderUsageEnv `expr:"usage"`
}

type ScriptedPriceProvider struct {
	vm  *vm.Program
	now func() time.Time
}

func NewScriptedPriceProvider(script string) (*ScriptedPriceProvider, error) {
	program, err := expr.Compile(script, expr.Env(PriceProviderEnv{}))
	if err != nil {
		return nil, fmt.Errorf("compile deepseek price provider script err: %w", err)
	}

	return &ScriptedPriceProvider{
		vm:  program,
		now: time.Now,
	}, nil
}

func (p *ScriptedPriceProvider) Provide(ctx context.Context, model string, usage chat.RecordTokenUsage) (TokenPrices, error) {
	now := time.Now()
	if p.now != nil {
		now = p.now()
	}
	runOut, err := expr.Run(p.vm, PriceProviderEnv{
		Model: model,
		Now:   now,
		Usage: toPriceProviderUsageEnv(usage),
	})
	if err != nil {
		return TokenPrices{}, fmt.Errorf("scripted price provider run err: %w", err)
	}

	// script output a string means error message
	errMsg, ok := runOut.(string)
	if ok {
		return TokenPrices{}, fmt.Errorf("scripted price provider error: %s", errMsg)
	}

	// output should be a map of all prices
	prices, ok := runOut.(map[string]any)
	if !ok {
		return TokenPrices{}, fmt.Errorf("scripted provider output is not map[string]any but %T", runOut)
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
