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

type Text2ImageLogStore interface {
	Create(ctx context.Context, log *schema.Text2ImageLog) error
}

type Dao struct {
	Closer misc.Closer

	LLMLogStore        LLMLogStore
	EmbeddingLogStore  EmbeddingLogStore
	Text2ImageLogStore Text2ImageLogStore
}

func NewDao(
	closer misc.Closer,
	llmLogStore LLMLogStore,
	embeddingLogStore EmbeddingLogStore,
	text2ImageLogStore Text2ImageLogStore,
) *Dao {
	return &Dao{
		Closer:             closer,
		LLMLogStore:        llmLogStore,
		EmbeddingLogStore:  embeddingLogStore,
		Text2ImageLogStore: text2ImageLogStore,
	}
}
