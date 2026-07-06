package summarizer

import (
	"context"

	llm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
)

type Summarizer interface {
	Summarize(ctx context.Context, text string) (string, error)
	SummarizeWith(ctx context.Context, provider llm.Provider, model string, text string) (string, error)
}
