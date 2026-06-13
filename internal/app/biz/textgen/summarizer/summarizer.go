package summarizer

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
)

type Summarizer interface {
	Summarize(ctx context.Context, text string) (string, error)
	SummarizeWith(ctx context.Context, provider chat.Provider, model string, text string) (string, error)
}
