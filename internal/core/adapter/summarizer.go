package adapter

import (
	"context"
)

type SummarizeOptionImpl struct {
	Provider string
	Prompt   string
	Model    string
	MinWord  int
	MaxWord  int
}

type SummarizeOption func(o *SummarizeOptionImpl)

func WithProvider(provider string) SummarizeOption {
	return func(o *SummarizeOptionImpl) {
		o.Provider = provider
	}
}

func WithModel(model string) SummarizeOption {
	return func(o *SummarizeOptionImpl) {
		o.Model = model
	}
}

func WithMinWord(minWord int) SummarizeOption {
	return func(o *SummarizeOptionImpl) {
		o.MinWord = minWord
	}
}

func WithMaxWord(maxWord int) SummarizeOption {
	return func(o *SummarizeOptionImpl) {
		o.MaxWord = maxWord
	}
}

func WithPrompt(prompt string) SummarizeOption {
	return func(o *SummarizeOptionImpl) {
		o.Prompt = prompt
	}
}

type Summarizer interface {
	Summarize(ctx context.Context, text string, opts ...SummarizeOption) (string, error)
}
