package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/shopspring/decimal"
)

type DashScopePriceProvider struct {
	vm *vm.Program
}

type DashScopePriceProviderEnv struct {
	Model string    `expr:"model"`
	Now   time.Time `expr:"now"`
}

func NewDashScopePriceProvider(script string) (TokenPricesProvider, error) {
	vm, err := expr.Compile(script, expr.Env(DashScopePriceProviderEnv{}))
	if err != nil {
		return nil, fmt.Errorf("compile dashscope price provider script err: %w", err)
	}

	return &DashScopePriceProvider{
		vm: vm,
	}, nil
}

func (p *DashScopePriceProvider) Provide(ctx context.Context, model string) (TokenPrices, error) {
	runOut, err := expr.Run(p.vm, DashScopePriceProviderEnv{
		Model: model,
		Now:   time.Now(),
	})
	if err != nil {
		return TokenPrices{}, fmt.Errorf("dashscope price provider run err: %w", err)
	}

	// output should be a map of all prices
	prices, ok := runOut.(map[string]any)
	if !ok {
		return TokenPrices{}, fmt.Errorf("dashscope provider output is not map[string]any but %T", runOut)
	}

	const promptKey = "prompt_1m"

	promptRaw, ok := prices[promptKey].(string)
	if !ok {
		return TokenPrices{}, fmt.Errorf("dashscope price provider missing prompt key")
	}

	promptPrice, err := decimal.NewFromString(promptRaw)
	if err != nil {
		return TokenPrices{}, fmt.Errorf("dashscope price provider prompt is not number")
	}

	return TokenPrices{
		PromptPrice: promptPrice,
	}, nil
}
