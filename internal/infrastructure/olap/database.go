package olap

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
)

type LLMLogStore interface {
	Create(ctx context.Context, log *schema.LLMLog) error
}

type EmbeddingLogStore interface {
	Create(ctx context.Context, log *schema.EmbeddingLog) error
}

type Dao struct {
	Closer misc.Closer

	LLMLogStore       LLMLogStore
	EmbeddingLogStore EmbeddingLogStore
}

func NewDao(
	closer misc.Closer,
	llmLogStore LLMLogStore,
	embeddingLogStore EmbeddingLogStore,
) *Dao {
	return &Dao{
		Closer:            closer,
		LLMLogStore:       llmLogStore,
		EmbeddingLogStore: embeddingLogStore,
	}
}
