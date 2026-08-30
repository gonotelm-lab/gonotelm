package billing

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	"github.com/shopspring/decimal"
)

type StandardMeter struct {
	mu        sync.RWMutex
	providers map[embedding.EmbeddingType]TokenPricesProvider
}

type StandardMeterConfig struct {
	DashScopeScript string
}

func NewStandardMeter(c StandardMeterConfig) (Meter, error) {
	meter := &StandardMeter{
		providers: make(map[embedding.EmbeddingType]TokenPricesProvider, 8),
	}

	if len(c.DashScopeScript) > 0 {
		dashScope, err := NewScriptedPriceProvider(c.DashScopeScript)
		if err != nil {
			return nil, fmt.Errorf("init dashscope script err: %w", err)
		}
		meter.SetProvider(embedding.EmbeddingDashScope, dashScope)
	}

	return meter, nil
}

func (m *StandardMeter) SetProvider(provider embedding.EmbeddingType, pricer TokenPricesProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider] = pricer
}

var _ Meter = &StandardMeter{}

func (m *StandardMeter) Calculate(
	ctx context.Context,
	provider embedding.EmbeddingType,
	model string,
	usage embedding.RecordTokenUsage,
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

	// 1M based
	promptTokens := decimal.NewFromInt(int64(usage.PromptTokens)).Div(millionUnit)

	promptPrice := promptTokens.Mul(prices.PromptPrice)

	final := decimal.Zero.Add(promptPrice)

	return &final, map[PriceDetailKey]decimal.Decimal{
		PromptPriceKey: promptPrice,
	}, nil
}
