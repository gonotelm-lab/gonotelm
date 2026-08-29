package billing

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/shopspring/decimal"
)

type StandardMeter struct {
	mu        sync.RWMutex
	providers map[chat.Provider]TokenPricesProvider
}

type StandardMeterConfig struct {
	DeepSeekScript string
	QwenScript     string
}

func NewStandardMeter(c StandardMeterConfig) (Meter, error) {
	meter := &StandardMeter{
		providers: make(map[chat.Provider]TokenPricesProvider, 8),
	}

	if len(c.DeepSeekScript) > 0 {
		deepSeek, err := NewScriptedPriceProvider(c.DeepSeekScript)
		if err != nil {
			return nil, fmt.Errorf("init deepseek script err: %w", err)
		}
		meter.SetProvider(chat.ProviderDeepSeek, deepSeek)
	}
	if len(c.QwenScript) > 0 {
		qwen, err := NewScriptedPriceProvider(c.QwenScript)
		if err != nil {
			return nil, fmt.Errorf("init qwen script err: %w", err)
		}
		meter.SetProvider(chat.ProviderQwen, qwen)
	}

	return meter, nil
}

func (m *StandardMeter) SetProvider(provider chat.Provider, pricer TokenPricesProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider] = pricer
}

var _ Meter = &StandardMeter{}

func (m *StandardMeter) Calculate(
	ctx context.Context,
	provider chat.Provider,
	model string,
	usage chat.RecordTokenUsage,
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
	cacheMissTokens := decimal.NewFromInt(int64(usage.PromptTokens - usage.PromptCachedTokens)).Div(millionUnit)
	cacheHitTokens := decimal.NewFromInt(int64(usage.PromptCachedTokens)).Div(millionUnit)
	outputTokens := decimal.NewFromInt(int64(usage.CompletionTokens)).Div(millionUnit)

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

