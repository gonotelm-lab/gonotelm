package summarizer

import (
	"context"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einoschema "github.com/cloudwego/eino/schema"
)

type summazierImpl struct {
	gateway *gateway.Gateway
	option  SummarizeOption
}

type SummarizeOption struct {
	Provider chat.Provider
	Model    string
}

func New(gateway *gateway.Gateway) Summarizer {
	return NewWithOption(gateway, SummarizeOption{})
}

func NewWithOption(gateway *gateway.Gateway, option SummarizeOption) Summarizer {
	return &summazierImpl{
		gateway: gateway,
		option:  option,
	}
}

func (s *summazierImpl) Summarize(ctx context.Context, text string) (string, error) {
	return s.SummarizeWith(ctx, s.option.Provider, s.option.Model, text)
}

func (s *summazierImpl) SummarizeWith(
	ctx context.Context,
	provider chat.Provider,
	model string,
	text string,
) (string, error) {
	lang := pkgcontext.GetLang(ctx)
	msg, err := prompts.RenderSummarizeMessage(ctx, text, lang)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "render summarize prompt failed, err=%v", err)
	}

	p, err := s.gateway.GetProvider(provider)
	if err != nil {
		return "", errors.Wrapf(errors.ErrParams, "get provider failed, err=%v", err)
	}

	opt := chat.WithModel(model)
	result, err := p.Generate(ctx, []*einoschema.Message{msg}, opt)
	if err != nil {
		return "", errors.Wrapf(errors.ErrLLM, "generate summary failed, err=%v", err)
	}

	return strings.TrimSpace(result.Content), nil
}
