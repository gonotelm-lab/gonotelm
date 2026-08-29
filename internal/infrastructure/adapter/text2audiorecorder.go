package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio/billing"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/shopspring/decimal"
)

const (
	usageCharacters        = "characters"
	usageInputTokens       = "input_tokens"
	usageCachedInputTokens = "cached_input_tokens"
	usageOutputTokens      = "output_tokens"

	priceCharacter = "character_prices"
)

type Text2AudioRecorderAdapter struct {
	store        olap.Text2AudioLogStore
	billingMeter billing.Meter
}

func NewText2AudioRecorderAdapter(store olap.Text2AudioLogStore, billingMeter billing.Meter) text2audio.Recorder {
	return &Text2AudioRecorderAdapter{store: store, billingMeter: billingMeter}
}

func (a *Text2AudioRecorderAdapter) Record(ctx context.Context, record *text2audio.Record) error {
	now := time.Now()
	log := &schema.Text2AudioLog{
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
	if record.Text != "" {
		log.Text = ptr(record.Text)
	}
	if record.Parameters != nil {
		log.Model = record.Parameters.Model
		if modelParameters, err := json.Marshal(record.Parameters); err == nil {
			log.ModelParameters = ptr(pkgstring.FromBytes(modelParameters))
		}
	}
	if record.Usage != nil {
		log.UsageDetails = toText2AudioUsageDetails(record.Usage)

		if a.billingMeter != nil {
			totalCost, costDetails, err := a.billingMeter.Calculate(ctx, record.Provider, log.Model, *record.Usage)
			if err != nil {
				slog.ErrorContext(ctx,
					"text2audio recorder billing meter failed to evaluate cost",
					slog.Any("err", err),
					slog.String("text2audio_provider", record.Provider.String()),
					slog.String("text2audio_model", log.Model),
				)
			} else if totalCost != nil {
				log.TotalCost = totalCost
				log.CostDetails = toText2AudioCostDetails(costDetails)
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
		return errors.WithMessagef(err, "text2audio record log failed")
	}

	return nil
}

func toText2AudioUsageDetails(usage *text2audio.RecordUsage) map[string]uint64 {
	if usage == nil {
		return nil
	}
	details := make(map[string]uint64, 5)
	if usage.Characters != nil {
		details[usageCharacters] = uint64(*usage.Characters)
	}
	if tu := usage.TokenUsage; tu != nil {
		details[usageInputTokens] = uint64(tu.InputTokens)
		details[usageCachedInputTokens] = uint64(tu.CachedInputTokens)
		details[usageOutputTokens] = uint64(tu.OutputTokens)
		details[usageTotalTokens] = uint64(tu.TotalTokens)
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func toText2AudioCostDetails(costDetails map[billing.PriceDetailKey]decimal.Decimal) map[string]decimal.Decimal {
	if len(costDetails) == 0 {
		return nil
	}
	out := make(map[string]decimal.Decimal, len(costDetails))
	if v, ok := costDetails[billing.CharacterPriceKey]; ok {
		out[priceCharacter] = v
	}
	if v, ok := costDetails[billing.CacheHitPriceKey]; ok {
		out[priceCachedToken] = v
	}
	if v, ok := costDetails[billing.CacheMissPriceKey]; ok {
		out[priceUncachedToken] = v
	}
	if v, ok := costDetails[billing.OutputPriceKey]; ok {
		out[priceOutputToken] = v
	}
	return out
}
