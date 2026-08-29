package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image/billing"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/shopspring/decimal"
)

const (
	usageOutputCount = "output_count"
	priceImage       = "image_prices"
)

type Text2ImageRecorderAdapter struct {
	store        olap.Text2ImageLogStore
	billingMeter billing.Meter
}

func NewText2ImageRecorderAdapter(store olap.Text2ImageLogStore, billingMeter billing.Meter) text2image.Recorder {
	return &Text2ImageRecorderAdapter{store: store, billingMeter: billingMeter}
}

func (a *Text2ImageRecorderAdapter) Record(ctx context.Context, record *text2image.Record) error {
	now := time.Now()
	log := &schema.Text2ImageLog{
		ID:             uuid.NewV7().String(),
		GroupID:        pkgcontext.GetSceneGroupId(ctx),
		TraceID:        pkgcontext.GetReqId(ctx).String(),
		UserID:         pkgcontext.GetUserId(ctx).String(),
		Scene:          string(record.Scene),
		ModelProvider:  record.Provider.String(),
		CallStartTime:  record.StartTime,
		CallFinishTime: record.EndTime,
		CreateTime:     now,
	}
	if record.Prompt != "" {
		log.Prompt = ptr(record.Prompt)
	}
	if record.Parameters != nil {
		log.Model = record.Parameters.Model
		if modelParameters, err := json.Marshal(record.Parameters); err == nil {
			log.ModelParameters = ptr(pkgstring.FromBytes(modelParameters))
		}
	}
	if record.Usage != nil {
		log.UsageDetails = map[string]uint64{
			usageOutputCount: uint64(record.Usage.OutputCount),
		}

		if a.billingMeter != nil {
			totalCost, costDetails, err := a.billingMeter.Calculate(ctx, record.Provider, log.Model, *record.Usage)
			if err != nil {
				slog.ErrorContext(ctx,
					"text2image recorder billing meter failed to evaluate cost",
					slog.Any("err", err),
					slog.String("text2image_provider", record.Provider.String()),
					slog.String("text2image_model", log.Model),
				)
			} else {
				log.TotalCost = totalCost
				log.CostDetails = map[string]decimal.Decimal{
					priceImage: costDetails[billing.ImagePriceKey],
				}
			}
		}
	}
	if len(record.Metadatas) > 0 {
		metadatas := make(map[string]string, len(record.Metadatas))
		for k, v := range record.Metadatas {
			metadatas[k] = fmt.Sprint(v)
		}
		log.Metadata = metadatas
	}
	if record.Error != nil {
		log.Error = ptr(record.Error.Error())
	}

	err := a.store.Create(ctx, log)
	if err != nil {
		return errors.WithMessagef(err, "text2image record log failed")
	}

	return nil
}
