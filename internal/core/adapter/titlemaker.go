package adapter

import (
	"context"
)

type MakeTitleOptionImpl struct {
	Provider string
	Prompt   string
	Model    string
	MinLen   int
	MaxLen   int
}

type MakeTitleOption func(o *MakeTitleOptionImpl)

func WithTitleProvider(provider string) MakeTitleOption {
	return func(o *MakeTitleOptionImpl) {
		o.Provider = provider
	}
}

func WithTitleModel(model string) MakeTitleOption {
	return func(o *MakeTitleOptionImpl) {
		o.Model = model
	}
}

func WithTitleMinLen(minLen int) MakeTitleOption {
	return func(o *MakeTitleOptionImpl) {
		o.MinLen = minLen
	}
}

func WithTitleMaxLen(maxLen int) MakeTitleOption {
	return func(o *MakeTitleOptionImpl) {
		o.MaxLen = maxLen
	}
}

func WithTitlePrompt(prompt string) MakeTitleOption {
	return func(o *MakeTitleOptionImpl) {
		o.Prompt = prompt
	}
}

type TitleMaker interface {
	MakeTitle(ctx context.Context, text string, opts ...MakeTitleOption) (string, error)
}
