package olap

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
)

type LLMLogStore interface {
	Create(ctx context.Context, log *schema.LLMLog) error
	Query(ctx context.Context, userId string, timeRange schema.TimeRange, extra *schema.ExtraQueryConditions) ([]*schema.LLMLog, error)
}

type EmbeddingLogStore interface {
	Create(ctx context.Context, log *schema.EmbeddingLog) error
	Query(ctx context.Context, userId string, timeRange schema.TimeRange, extra *schema.ExtraQueryConditions) ([]*schema.EmbeddingLog, error)
}

type Text2ImageLogStore interface {
	Create(ctx context.Context, log *schema.Text2ImageLog) error
	Query(ctx context.Context, userId string, timeRange schema.TimeRange, extra *schema.ExtraQueryConditions) ([]*schema.Text2ImageLog, error)
}

type Text2AudioLogStore interface {
	Create(ctx context.Context, log *schema.Text2AudioLog) error
	Query(ctx context.Context, userId string, timeRange schema.TimeRange, extra *schema.ExtraQueryConditions) ([]*schema.Text2AudioLog, error)
}

type Dao struct {
	Closer misc.Closer

	LLMLogStore        LLMLogStore
	EmbeddingLogStore  EmbeddingLogStore
	Text2ImageLogStore Text2ImageLogStore
	Text2AudioLogStore Text2AudioLogStore
}

func NewDao(
	closer misc.Closer,
	llmLogStore LLMLogStore,
	embeddingLogStore EmbeddingLogStore,
	text2ImageLogStore Text2ImageLogStore,
	text2AudioLogStore Text2AudioLogStore,
) *Dao {
	return &Dao{
		Closer:             closer,
		LLMLogStore:        llmLogStore,
		EmbeddingLogStore:  embeddingLogStore,
		Text2ImageLogStore: text2ImageLogStore,
		Text2AudioLogStore: text2AudioLogStore,
	}
}
