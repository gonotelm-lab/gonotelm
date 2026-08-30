package billing

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/shopspring/decimal"
)

type CharacterPrices struct {
	// CharacterPrice per 10k characters
	CharacterPrice decimal.Decimal
}

type CharacterPricesProvider interface {
	Provide(ctx context.Context, model string, usage text2audio.RecordUsage) (CharacterPrices, error)
}

type CharacterBasedMeter struct {
	mu        sync.RWMutex
	providers map[text2audio.Text2AudioProvider]CharacterPricesProvider
}

func NewCharacterBasedMeter() *CharacterBasedMeter {
	return &CharacterBasedMeter{
		providers: make(map[text2audio.Text2AudioProvider]CharacterPricesProvider, 8),
	}
}

func (m *CharacterBasedMeter) SetProvider(provider text2audio.Text2AudioProvider, pricer CharacterPricesProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider] = pricer
}

var _ Meter = &CharacterBasedMeter{}

func (m *CharacterBasedMeter) Calculate(
	ctx context.Context,
	provider text2audio.Text2AudioProvider,
	model string,
	usage text2audio.RecordUsage,
) (*decimal.Decimal, map[PriceDetailKey]decimal.Decimal, error) {
	if usage.Characters == nil {
		return nil, nil, ErrMissingCharacters
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

	// 10K characters based
	characters := decimal.NewFromInt(*usage.Characters).Div(tenThousandUnit)
	characterPrice := characters.Mul(prices.CharacterPrice)
	final := decimal.Zero.Add(characterPrice)

	return &final, map[PriceDetailKey]decimal.Decimal{
		CharacterPriceKey: characterPrice,
	}, nil
}
