package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/shopspring/decimal"
)

const scriptedPromptKey = "prompt_1m"

type PriceProviderUsageEnv struct {
	PromptTokens     int `expr:"prompt_tokens"`
	CompletionTokens int `expr:"completion_tokens"`
	TotalTokens      int `expr:"total_tokens"`
}

func toPriceProviderUsageEnv(usage embedding.RecordTokenUsage) PriceProviderUsageEnv {
	return PriceProviderUsageEnv{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

type PriceProviderEnv struct {
	Model string                `expr:"model"`
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
		return nil, fmt.Errorf("compile scripted price provider script err: %w", err)
	}

	return &ScriptedPriceProvider{
		vm:  program,
		now: time.Now,
	}, nil
}

func (p *ScriptedPriceProvider) Provide(ctx context.Context, model string, usage embedding.RecordTokenUsage) (TokenPrices, error) {
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
	if errMsg, ok := runOut.(string); ok {
		return TokenPrices{}, fmt.Errorf("scripted price provider error: %s", errMsg)
	}

	prices, ok := runOut.(map[string]any)
	if !ok {
		return TokenPrices{}, fmt.Errorf("scripted provider output is not map[string]any but %T", runOut)
	}

	promptRaw, ok := prices[scriptedPromptKey].(string)
	if !ok {
		return TokenPrices{}, ErrMissingPromptPrice
	}

	promptPrice, err := decimal.NewFromString(promptRaw)
	if err != nil {
		return TokenPrices{}, ErrPromptNotNumber
	}

	return TokenPrices{
		PromptPrice: promptPrice,
	}, nil
}
