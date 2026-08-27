package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat/billing"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/shopspring/decimal"
)

const (
	usagePromptTokens       = "prompt_tokens"
	usagePromptCachedTokens = "prompt_cached_tokens"
	usageCompletionTokens   = "completion_tokens"
	usageTotalTokens        = "total_tokens"
)

const (
	priceCachedToken   = "cached_token_prices"
	priceUncachedToken = "uncached_token_prices"
	priceOutputToken   = "output_prices"
)

type LLMRecorderAdapter struct {
	store        olap.LLMLogStore
	billingMeter billing.Meter
}

func NewLLMRecorderAdapter(store olap.LLMLogStore, billingMeter billing.Meter) chat.Recorder {
	return &LLMRecorderAdapter{store: store, billingMeter: billingMeter}
}

func (a *LLMRecorderAdapter) Record(ctx context.Context, record *chat.Record) error {
	now := time.Now()
	log := &schema.LLMLog{
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
	if record.Parameters != nil {
		log.Model = record.Parameters.Model
		if modelParameters, err := json.Marshal(record.Parameters); err == nil {
			log.ModelParameters = ptr(pkgstring.FromBytes(modelParameters))
		}
	}
	if len(record.Input) > 0 {
		if input, err := json.Marshal(record.Input); err == nil {
			log.Input = ptr(pkgstring.FromBytes(input))
		}
	}
	if record.Output != nil {
		if output, err := json.Marshal(record.Output); err == nil {
			log.Output = ptr(pkgstring.FromBytes(output))
		}
		log.ToolCalls = toLLMLogToolCalls(record.Output.ToolCalls)
	}
	if len(record.InputTools) > 0 {
		log.ToolDefinitions = toToolDefinitions(record.InputTools)
	}
	if record.Usage != nil {
		log.UsageDetails = map[string]uint64{
			usagePromptTokens:       uint64(record.Usage.PromptTokens),
			usagePromptCachedTokens: uint64(record.Usage.PromptCachedTokens),
			usageCompletionTokens:   uint64(record.Usage.CompletionTokens),
			usageTotalTokens:        uint64(record.Usage.TotalTokens),
		}

		if a.billingMeter != nil {
			totalCost, costDetails, err := a.billingMeter.Calculate(ctx, record.Provider, log.Model, *record.Usage)
			if err != nil {
				slog.ErrorContext(ctx,
					"llm recorder billing meter failed to evaluate cost",
					slog.Any("err", err),
					slog.String("llm_provider", record.Provider.String()),
					slog.String("llm_model", log.Model),
				)
			} else {
				log.TotalCost = totalCost
				log.CostDetails = map[string]decimal.Decimal{
					priceCachedToken:   costDetails[billing.CacheHitPriceKey],
					priceUncachedToken: costDetails[billing.CacheMissPriceKey],
					priceOutputToken:   costDetails[billing.OutputPriceKey],
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
		return errors.WithMessagef(err, "llm record log failed")
	}

	return nil
}

func toLLMLogToolCalls(calls []*chat.RecordMessageToolCall) []schema.LLMLogToolCall {
	logCalls := make([]schema.LLMLogToolCall, 0, len(calls))
	for _, call := range calls {
		if call == nil {
			continue
		}
		logCalls = append(logCalls, schema.LLMLogToolCall{
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}
	return logCalls
}

func toToolDefinitions(tools []*chat.RecordInputTool) map[string]string {
	definitions := make(map[string]string, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if definition, err := json.Marshal(tool); err == nil {
			definitions[tool.Name] = pkgstring.FromBytes(definition)
		}
	}
	return definitions
}

func ptr[T any](t T) *T {
	return &t
}
