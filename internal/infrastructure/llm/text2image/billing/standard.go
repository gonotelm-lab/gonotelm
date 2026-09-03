package billing

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/shopspring/decimal"
)

type StandardMeter struct {
	mu        sync.RWMutex
	providers map[text2image.Text2ImageProvider]ImagePricesProvider
}

type StandardMeterConfig struct {
	QwenScript string
}

func NewStandardMeter(c StandardMeterConfig) (Meter, error) {
	meter := &StandardMeter{
		providers: make(map[text2image.Text2ImageProvider]ImagePricesProvider, 8),
	}

	if len(c.QwenScript) > 0 {
		qwen, err := NewScriptedPriceProvider(c.QwenScript)
		if err != nil {
			return nil, fmt.Errorf("init qwen script err: %w", err)
		}
		meter.SetProvider(text2image.Text2ImageQwen, qwen)
	}

	return meter, nil
}

func (m *StandardMeter) SetProvider(provider text2image.Text2ImageProvider, pricer ImagePricesProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider] = pricer
}

var _ Meter = &StandardMeter{}

func (m *StandardMeter) Calculate(
	ctx context.Context,
	provider text2image.Text2ImageProvider,
	model string,
	usage text2image.RecordUsage,
) (*decimal.Decimal, map[PriceDetailKey]decimal.Decimal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	priceProvider, ok := m.providers[provider]
	if !ok {
		return nil, nil, fmt.Errorf("%s: %w", provider, ErrPriceProviderNotFound)
	}

	prices, err := priceProvider.Provide(ctx, model, usage)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", model, err)
	}

	outputCount := decimal.NewFromInt(int64(usage.OutputCount))
	imagePrice := outputCount.Mul(prices.ImagePrice)
	final := decimal.Zero.Add(imagePrice)

	return &final, map[PriceDetailKey]decimal.Decimal{
		ImagePriceKey: imagePrice,
	}, nil
}
