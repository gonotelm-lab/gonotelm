package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/shopspring/decimal"
)

const scriptedImageKey = "image"

type PriceProviderUsageEnv struct {
	OutputCount int `expr:"output_count"`
}

func toPriceProviderUsageEnv(usage text2image.RecordUsage) PriceProviderUsageEnv {
	return PriceProviderUsageEnv{
		OutputCount: usage.OutputCount,
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

func (p *ScriptedPriceProvider) Provide(ctx context.Context, model string, usage text2image.RecordUsage) (ImagePrices, error) {
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
		return ImagePrices{}, fmt.Errorf("scripted price provider run err: %w", err)
	}

	if errMsg, ok := runOut.(string); ok {
		return ImagePrices{}, fmt.Errorf("scripted price provider error: %s", errMsg)
	}

	prices, ok := runOut.(map[string]any)
	if !ok {
		return ImagePrices{}, fmt.Errorf("scripted provider output is not map[string]any but %T", runOut)
	}

	imageRaw, ok := prices[scriptedImageKey].(string)
	if !ok {
		return ImagePrices{}, ErrMissingImagePrice
	}

	imagePrice, err := decimal.NewFromString(imageRaw)
	if err != nil {
		return ImagePrices{}, ErrImageNotNumber
	}

	return ImagePrices{
		ImagePrice: imagePrice,
	}, nil
}
