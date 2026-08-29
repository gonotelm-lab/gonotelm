package billing

import (
	"context"
	"errors"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"

	"github.com/shopspring/decimal"
)

var (
	ErrPriceProviderNotFound = errors.New("price provider not found")
	ErrModelNotFound         = errors.New("model not found")

	ErrMissingCacheHitPrice  = errors.New("scripted price provider missing cache hit key")
	ErrMissingCacheMissPrice = errors.New("scripted price provider missing cache missed key")
	ErrMissingOutputPrice    = errors.New("scripted price provider missing output key")
	ErrCacheHitNotNumber     = errors.New("scripted price provider cache hit is not number")
	ErrCacheMissNotNumber    = errors.New("scripted price provider cache miss is not number")
	ErrOutputNotNumber       = errors.New("scripted price provider output is not number")
)

var millionUnit = decimal.NewFromInt(1_000_000) // 1M

type PriceDetailKey string

const (
	CacheHitPriceKey  PriceDetailKey = "cache_hit_price"
	CacheMissPriceKey PriceDetailKey = "cache_miss_price"
	OutputPriceKey    PriceDetailKey = "output_key"
)

type Meter interface {
	Calculate(
		ctx context.Context,
		provider chat.Provider,
		model string,
		usage chat.RecordTokenUsage,
	) (*decimal.Decimal, map[PriceDetailKey]decimal.Decimal, error)
}

// 1M token prices
type TokenPrices struct {
	CacheHitPrice  decimal.Decimal
	CacheMissPrice decimal.Decimal
	OutputPrice    decimal.Decimal
}

type TokenPricesProvider interface {
	Provide(ctx context.Context, model string, usage chat.RecordTokenUsage) (TokenPrices, error)
}
