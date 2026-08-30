package billing

import (
	"context"
	"errors"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"

	"github.com/shopspring/decimal"
)

var (
	ErrPriceProviderNotFound = errors.New("price provider not found")
	ErrModelNotFound         = errors.New("model not found")

	ErrMissingCharacters = errors.New("character-based meter requires characters usage")
	ErrMissingTokenUsage = errors.New("token-based meter requires token usage")

	ErrMissingCharacterPrice = errors.New("scripted price provider missing character_10k key")
	ErrCharacterNotNumber    = errors.New("scripted price provider character_10k is not number")

	ErrMissingCacheHitPrice  = errors.New("scripted price provider missing cache hit key")
	ErrMissingCacheMissPrice = errors.New("scripted price provider missing cache miss key")
	ErrMissingOutputPrice    = errors.New("scripted price provider missing output key")
	ErrCacheHitNotNumber     = errors.New("scripted price provider cache hit is not number")
	ErrCacheMissNotNumber    = errors.New("scripted price provider cache miss is not number")
	ErrOutputNotNumber       = errors.New("scripted price provider output is not number")
)

var (
	millionUnit     = decimal.NewFromInt(1_000_000) // 1M tokens
	tenThousandUnit = decimal.NewFromInt(10_000)    // 10K characters
)

type PriceDetailKey string

const (
	CharacterPriceKey PriceDetailKey = "character_price"
	CacheHitPriceKey  PriceDetailKey = "cache_hit_price"
	CacheMissPriceKey PriceDetailKey = "cache_miss_price"
	OutputPriceKey    PriceDetailKey = "output_price"
)

type Meter interface {
	Calculate(
		ctx context.Context,
		provider text2audio.Text2AudioProvider,
		model string,
		usage text2audio.RecordUsage,
	) (*decimal.Decimal, map[PriceDetailKey]decimal.Decimal, error)
}
