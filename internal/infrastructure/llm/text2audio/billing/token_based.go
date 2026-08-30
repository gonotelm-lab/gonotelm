package billing

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/shopspring/decimal"
)

type TokenPrices struct {
	CacheHitPrice  decimal.Decimal
	CacheMissPrice decimal.Decimal
	OutputPrice    decimal.Decimal
}

type TokenPricesProvider interface {
	Provide(ctx context.Context, model string, usage text2audio.RecordUsage) (TokenPrices, error)
}

type TokenBasedMeter struct {
	mu        sync.RWMutex
	providers map[text2audio.Text2AudioProvider]TokenPricesProvider
}

func NewTokenBasedMeter() *TokenBasedMeter {
	return &TokenBasedMeter{
		providers: make(map[text2audio.Text2AudioProvider]TokenPricesProvider, 8),
	}
}

func (m *TokenBasedMeter) SetProvider(provider text2audio.Text2AudioProvider, pricer TokenPricesProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider] = pricer
}

var _ Meter = &TokenBasedMeter{}

func (m *TokenBasedMeter) Calculate(
	ctx context.Context,
	provider text2audio.Text2AudioProvider,
	model string,
	usage text2audio.RecordUsage,
) (*decimal.Decimal, map[PriceDetailKey]decimal.Decimal, error) {
	if usage.TokenUsage == nil {
		return nil, nil, ErrMissingTokenUsage
	}

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

	tu := usage.TokenUsage
	cacheMissTokens := decimal.NewFromInt(tu.InputTokens - tu.CachedInputTokens).Div(millionUnit)
	cacheHitTokens := decimal.NewFromInt(tu.CachedInputTokens).Div(millionUnit)
	outputTokens := decimal.NewFromInt(tu.OutputTokens).Div(millionUnit)

	cacheMissPrice := cacheMissTokens.Mul(prices.CacheMissPrice)
	cacheHitPrice := cacheHitTokens.Mul(prices.CacheHitPrice)
	outputPrice := outputTokens.Mul(prices.OutputPrice)

	final := decimal.Zero.
		Add(cacheHitPrice).
		Add(cacheMissPrice).
		Add(outputPrice)

	return &final, map[PriceDetailKey]decimal.Decimal{
		CacheHitPriceKey:  cacheHitPrice,
		CacheMissPriceKey: cacheMissPrice,
		OutputPriceKey:    outputPrice,
	}, nil
}
