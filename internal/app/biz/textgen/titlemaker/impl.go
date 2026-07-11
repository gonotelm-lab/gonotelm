package titlemaker

import (
	"context"
	"strings"

	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	llm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type titlemakerImpl struct {
	gateway *chat.Gateway
	option  GenerateOption
	prompt  *bizprompt.Prompt
}

type GenerateOption struct {
	Provider llm.Provider
	Model    string
}

func New(gateway *chat.Gateway, prompt *bizprompt.Prompt) Maker {
	return NewWithOption(gateway, GenerateOption{}, prompt)
}

func NewWithOption(gateway *chat.Gateway, option GenerateOption, prompt *bizprompt.Prompt) Maker {
	return &titlemakerImpl{
		gateway: gateway,
		option:  option,
		prompt:  prompt,
	}
}

func (t *titlemakerImpl) Generate(ctx context.Context, text string) (string, error) {
	return t.GenerateWith(ctx, t.option.Provider, t.option.Model, text)
}

func (t *titlemakerImpl) GenerateWith(ctx context.Context, provider llm.Provider, model string, text string) (string, error) {
	lang := pkgcontext.GetLang(ctx)
	msgs, err := t.prompt.RenderTitleMakerMessage(ctx, text, lang)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "render title maker prompt failed, err=%v", err)
	}

	p, err := t.gateway.GetProvider(provider)
	if err != nil {
		return "", errors.Wrapf(errors.ErrParams, "get provider failed, err=%v", err)
	}

	opt := chat.WithModel(model)
	result, err := p.Generate(ctx, msgs, opt)
	if err != nil {
		return "", errors.Wrapf(errors.ErrLLM, "generate title failed, err=%v", err)
	}

	return strings.TrimSpace(result.Content), nil
}
