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

const titleMakerPromptTemplate = `
Write a one-sentence title for the text in <content>, in the same language as the content.

Requirements:
- Capture the core theme and key information; keep it accurate, natural, and readable.
- No marketing hype; stay faithful—do not invent facts absent from the source.
- Output only the title text: no explanations, bullets, quotation marks, or wrappers; no line breaks.
- Prefer %d-%d characters.
- If information is thin, produce a conservative but informative title without speculation.

<content>
%s
</content>`

const (
	defaultTitleMinLen = 10
	defaultTitleMaxLen = 15
)

type TitleMakerImpl struct {
	provider chat.Provider
	model    string
	llm      *chat.Gateway
}

func NewTitleMaker(gw *chat.Gateway, provider chat.Provider, model string) adapter.TitleMaker {
	return &TitleMakerImpl{
		provider: provider,
		model:    model,
		llm:      gw,
	}
}

func (t *TitleMakerImpl) MakeTitle(
	ctx context.Context,
	text string,
	opts ...adapter.MakeTitleOption,
) (string, error) {
	opt := adapter.MakeTitleOptionImpl{
		MinLen: defaultTitleMinLen,
		MaxLen: defaultTitleMaxLen,
	}
	for _, o := range opts {
		o(&opt)
	}

	provider := t.provider
	if opt.Provider != "" {
		provider = chat.Provider(opt.Provider)
	}

	model := t.model
	if opt.Model != "" {
		model = opt.Model
	}

	var prompt string
	if opt.Prompt != "" {
		prompt = opt.Prompt
	} else {
		prompt = fmt.Sprintf(titleMakerPromptTemplate, opt.MinLen, opt.MaxLen, text)
	}

	tcm, err := t.llm.GetChatModel(provider)
	if err != nil {
		return "", errors.Wrapf(errors.ErrParams, "get provider failed, err=%v", err)
	}

	result, err := tcm.Generate(ctx,
		[]*einoschema.Message{
			{
				Role:    einoschema.User,
				Content: prompt,
			},
		},
		chat.WithModel(model),
		chat.WithThinking(provider, false),
	)
	if err != nil {
		return "", errors.WithMessagef(err, "generate title failed, err=%v", err)
	}

	return strings.TrimSpace(result.Content), nil
}
