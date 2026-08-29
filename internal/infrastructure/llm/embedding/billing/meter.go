package billing

import (
	"context"
	"errors"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"

	"github.com/shopspring/decimal"
)

var (
	ErrPriceProviderNotFound = errors.New("price provider not found")
	ErrModelNotFound         = errors.New("model not found")

	ErrMissingPromptPrice = errors.New("scripted price provider missing prompt key")
	ErrPromptNotNumber    = errors.New("scripted price provider prompt is not number")
)

var millionUnit = decimal.NewFromInt(1_000_000) // 1M

type PriceDetailKey string

const (
	PromptPriceKey PriceDetailKey = "prompt_price"
)

type Meter interface {
	Calculate(
		ctx context.Context,
		provider embedding.EmbeddingType,
		model string,
		usage embedding.RecordTokenUsage,
	) (*decimal.Decimal, map[PriceDetailKey]decimal.Decimal, error)
}

// 1M token prices
type TokenPrices struct {
	PromptPrice decimal.Decimal
}

type TokenPricesProvider interface {
	Provide(ctx context.Context, model string, usage embedding.RecordTokenUsage) (TokenPrices, error)
}
