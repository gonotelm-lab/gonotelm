package billing

import (
	"context"
	"errors"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"

	"github.com/shopspring/decimal"
)

var (
	ErrPriceProviderNotFound = errors.New("price provider not found")
	ErrModelNotFound         = errors.New("model not found")

	ErrMissingImagePrice = errors.New("scripted price provider missing image key")
	ErrImageNotNumber    = errors.New("scripted price provider image is not number")
)

type PriceDetailKey string

const (
	ImagePriceKey PriceDetailKey = "image_price"
)

type Meter interface {
	Calculate(
		ctx context.Context,
		provider text2image.Text2ImageProvider,
		model string,
		usage text2image.RecordUsage,
	) (*decimal.Decimal, map[PriceDetailKey]decimal.Decimal, error)
}

// ImagePrices is per-image prices.
type ImagePrices struct {
	ImagePrice decimal.Decimal
}

type ImagePricesProvider interface {
	Provide(ctx context.Context, model string, usage text2image.RecordUsage) (ImagePrices, error)
}
