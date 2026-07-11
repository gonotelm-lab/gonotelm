package summarizer

import (
	"context"
	"strings"

	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	llm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type summazierImpl struct {
	gateway *chat.Gateway
	option  SummarizeOption
	prompt  *bizprompt.Prompt
}

type SummarizeOption struct {
	Provider llm.Provider
	Model    string
}

func New(gateway *chat.Gateway, prompt *bizprompt.Prompt) Summarizer {
	return NewWithOption(gateway, SummarizeOption{}, prompt)
}

func NewWithOption(gateway *chat.Gateway, option SummarizeOption, prompt *bizprompt.Prompt) Summarizer {
	return &summazierImpl{
		gateway: gateway,
		option:  option,
		prompt:  prompt,
	}
}

func (s *summazierImpl) Summarize(ctx context.Context, text string) (string, error) {
	return s.SummarizeWith(ctx, s.option.Provider, s.option.Model, text)
}

func (s *summazierImpl) SummarizeWith(
	ctx context.Context,
	provider llm.Provider,
	model string,
	text string,
) (string, error) {
	lang := pkgcontext.GetLang(ctx)
	msgs, err := s.prompt.RenderSummarizeMessage(ctx, text, lang)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "render summarize prompt failed, err=%v", err)
	}

	p, err := s.gateway.GetProvider(provider)
	if err != nil {
		return "", errors.Wrapf(errors.ErrParams, "get provider failed, err=%v", err)
	}

	opt := chat.WithModel(model)
	result, err := p.Generate(ctx, msgs, opt)
	if err != nil {
		return "", errors.Wrapf(errors.ErrLLM, "generate summary failed, err=%v", err)
	}

	return strings.TrimSpace(result.Content), nil
}
