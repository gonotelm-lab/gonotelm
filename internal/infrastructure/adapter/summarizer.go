package adapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

const summarizePromptTemplate = `
Summarize the text in <content> in the same language as the content.

Requirements:
- Dense summary of the main theme, key conclusions, and salient facts (entities, numbers, times) when present.
- Drop preamble, repetition, and weakly related details.
- Stay faithful; do not invent or extrapolate.
- Output only the summary body: no title, bullets, explanations, or wrappers.
- Use 3-5 sentences; prefer %d-%d characters.
- If information is thin, summarize conservatively without speculation.

<content>
%s
</content>`

const (
	defaultMinWord = 60
	defaultMaxWord = 150
)

type SummarizerImpl struct {
	provider chat.Provider
	model    string
	llm      *chat.Gateway
}

func NewSummarizer(gw *chat.Gateway, provider chat.Provider, model string) adapter.Summarizer {
	return &SummarizerImpl{
		provider: provider,
		model:    model,
		llm:      gw,
	}
}

func (s *SummarizerImpl) Summarize(
	ctx context.Context,
	text string,
	opts ...adapter.SummarizeOption,
) (string, error) {
	opt := adapter.SummarizeOptionImpl{
		MinWord: defaultMinWord,
		MaxWord: defaultMaxWord,
	}
	for _, o := range opts {
		o(&opt)
	}

	provider := s.provider
	if opt.Provider != "" {
		provider = chat.Provider(opt.Provider)
	}

	model := s.model
	if opt.Model != "" {
		model = opt.Model
	}

	var prompt string
	if opt.Prompt != "" {
		prompt = opt.Prompt
	} else {
		prompt = fmt.Sprintf(summarizePromptTemplate, opt.MinWord, opt.MaxWord, text)
	}

	tcm, err := s.llm.GetChatModel(provider)
	if err != nil {
		return "", errors.Wrapf(errors.ErrParams, "get provider failed, err=%v", err)
	}

	result, err := tcm.Generate(ctx, []*einoschema.Message{
		{
			Role:    einoschema.User,
			Content: prompt,
		},
	}, chat.WithModel(model), chat.WithThinking(provider, false))
	if err != nil {
		return "", errors.WithMessagef(err, "generate summary failed, err=%v", err)
	}

	return strings.TrimSpace(result.Content), nil
}
