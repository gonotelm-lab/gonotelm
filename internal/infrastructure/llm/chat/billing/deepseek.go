package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/shopspring/decimal"
)

type DeepSeekPriceProvider struct {
	vm *vm.Program
}

type DeepSeekPriceProviderEnv struct {
	Model string    `expr:"model"` // model name
	Now   time.Time `expr:"now"`
}

func NewDeepSeekPriceProvider(script string) (TokenPricesProvider, error) {
	vm, err := expr.Compile(script, expr.Env(DeepSeekPriceProviderEnv{}))
	if err != nil {
		return nil, fmt.Errorf("compile deepseek price provider script err: %w", err)
	}

	return &DeepSeekPriceProvider{
		vm: vm,
	}, nil
}

func (p *DeepSeekPriceProvider) Provide(ctx context.Context, model string) (TokenPrices, error) {
	runOut, err := expr.Run(p.vm, DeepSeekPriceProviderEnv{
		Model: model,
		Now:   time.Now(),
	})
	if err != nil {
		return TokenPrices{}, fmt.Errorf("deepseek price provider run err: %w", err)
	}

	// output should be a map of all prices
	prices, ok := runOut.(map[string]any)
	if !ok {
		return TokenPrices{}, fmt.Errorf("deepseek provider output is not map[string]any but %T", runOut)
	}

	// and it should be map[string]any, any is of type string
	const (
		cacheHitKey  = "cache_hit_1m"
		cacheMissKey = "cache_miss_1m"
		outputKey    = "output_1m"
	)

	ch, ok := prices[cacheHitKey].(string)
	if !ok {
		return TokenPrices{}, fmt.Errorf("deepseek price provider missing cache hit key")
	}
	cm, ok := prices[cacheMissKey].(string)
	if !ok {
		return TokenPrices{}, fmt.Errorf("deepseek price provider missing cache missed key")
	}
	out, ok := prices[outputKey].(string)
	if !ok {
		return TokenPrices{}, fmt.Errorf("deepseek price provider missing output key")
	}

	cacheHit, err := decimal.NewFromString(ch)
	if err != nil {
		return TokenPrices{}, fmt.Errorf("deepseek price provider cache hit is not number")
	}
	cacheMiss, err := decimal.NewFromString(cm)
	if err != nil {
		return TokenPrices{}, fmt.Errorf("deepseek price provider cache miss is not number")
	}
	output, err := decimal.NewFromString(out)
	if err != nil {
		return TokenPrices{}, fmt.Errorf("deepseek price provider output is not number")
	}

	return TokenPrices{
		CacheHitPrice:  cacheHit,
		CacheMissPrice: cacheMiss,
		OutputPrice:    output,
	}, nil
}
