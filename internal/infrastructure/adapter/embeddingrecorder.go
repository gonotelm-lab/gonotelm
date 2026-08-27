package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type EmbeddingRecorderAdapter struct {
	store olap.EmbeddingLogStore
}

func NewEmbeddingRecorderAdapter(store olap.EmbeddingLogStore) embedding.Recorder {
	return &EmbeddingRecorderAdapter{store: store}
}

func (a *EmbeddingRecorderAdapter) Record(ctx context.Context, record *embedding.Record) error {
	now := time.Now()
	log := &schema.EmbeddingLog{
		ID:             uuid.NewV7().String(),
		GroupID:        pkgcontext.GetSceneGroupId(ctx),
		TraceID:        pkgcontext.GetReqId(ctx).String(),
		UserID:         pkgcontext.GetUserId(ctx).String(),
		Scene:          string(record.Scene),
		ModelProvider:  record.Provider.String(),
		CallStartTime:  record.StartTime,
		CallFinishTime: record.EndTime,
		InputCount:     uint32(len(record.Input)),
		CreateTime:     now,
	}
	if record.Parameters != nil {
		log.Model = record.Parameters.Model
		if modelParameters, err := json.Marshal(record.Parameters); err == nil {
			log.ModelParameters = ptr(pkgstring.FromBytes(modelParameters))
		}
	}
	if record.Output != nil {
		log.EmbeddingCount = uint32(record.Output.Count)
		log.EmbeddingDimensions = uint32(record.Output.Dimensions)
	}
	if record.Usage != nil {
		log.UsageDetails = map[string]uint64{
			usagePromptTokens:     uint64(record.Usage.PromptTokens),
			usageCompletionTokens: uint64(record.Usage.CompletionTokens),
			usageTotalTokens:      uint64(record.Usage.TotalTokens),
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
		return errors.WithMessagef(err, "embedding record log failed")
	}

	return nil
}
