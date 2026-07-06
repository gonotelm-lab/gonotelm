package summarizer

import (
	"context"

	llm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
)

type mockSummarizer struct{}

func NewMockSummarizer() Summarizer {
	return &mockSummarizer{}
}

func (m *mockSummarizer) Summarize(ctx context.Context, text string) (string, error) {
	return "mock summary", nil
}

func (m *mockSummarizer) SummarizeWith(
	ctx context.Context,
	provider llm.Provider,
	model string,
	text string,
) (string, error) {
	return m.Summarize(ctx, text)
}
