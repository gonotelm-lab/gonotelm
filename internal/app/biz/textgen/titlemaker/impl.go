package titlemaker

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

type titlemakerImpl struct {
	gateway *gateway.Gateway
	option  GenerateOption
}

type GenerateOption struct {
	Provider chat.Provider
	Model    string
}

func New(gateway *gateway.Gateway) Maker {
	return NewWithOption(gateway, GenerateOption{})
}

func NewWithOption(gateway *gateway.Gateway, option GenerateOption) Maker {
	return &titlemakerImpl{
		gateway: gateway,
		option:  option,
	}
}

func (t *titlemakerImpl) Generate(ctx context.Context, text string) (string, error) {
	return t.GenerateWith(ctx, t.option.Provider, t.option.Model, text)
}

func (t *titlemakerImpl) GenerateWith(ctx context.Context, provider chat.Provider, model string, text string) (string, error) {
	lang := pkgcontext.GetLang(ctx)
	msg, err := prompts.TitleMakerMessage(ctx, text, lang)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "render title maker prompt failed, err=%v", err)
	}

	p, err := t.gateway.GetProvider(provider)
	if err != nil {
		return "", errors.Wrapf(errors.ErrParams, "get provider failed, err=%v", err)
	}

	opt := chat.BuildLLMModelOption(model)
	result, err := p.Generate(ctx, []*einoschema.Message{msg}, opt)
	if err != nil {
		return "", errors.Wrapf(errors.ErrLLM, "generate title failed, err=%v", err)
	}

	return strings.TrimSpace(result.Content), nil
}
