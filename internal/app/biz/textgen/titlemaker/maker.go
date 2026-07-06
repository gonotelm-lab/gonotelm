package titlemaker

import (
	"context"

	llm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
)

type Maker interface {
	Generate(ctx context.Context, text string) (string, error)
	GenerateWith(ctx context.Context, provider llm.Provider, model string, text string) (string, error)
}
