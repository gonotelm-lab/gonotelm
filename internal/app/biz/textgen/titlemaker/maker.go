package titlemaker

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
)

type Maker interface {
	Generate(ctx context.Context, text string) (string, error)
	GenerateWith(ctx context.Context, provider chat.Provider, model string, text string) (string, error)
}
