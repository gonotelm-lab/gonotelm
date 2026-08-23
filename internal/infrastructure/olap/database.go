package olap

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
)

type LLMLogStore interface {
	Create(ctx context.Context, log *schema.LLMLog) error
}

type Dao struct {
	Closer misc.Closer

	LLMLogStore LLMLogStore
}

func NewDao(closer misc.Closer, llmLogStore LLMLogStore) *Dao {
	return &Dao{
		Closer:      closer,
		LLMLogStore: llmLogStore,
	}
}
