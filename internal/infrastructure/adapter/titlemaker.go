package adapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

const titleMakerPromptTemplate = `
你是一个标题生成助手，擅长从文本中提炼主线主题并生成高信息密度标题。

# 任务

- 基于输入文本，生成一句话标题，概括核心主题与关键信息。
- 标题应准确、自然、可读，不使用夸张营销表达。
- 保持忠实，不得编造原文未出现的事实。

# 输出约束

- 仅输出标题正文，不要解释、不要分点、不要引号、不要前后缀。
- 标题必须是一句话，不换行。
- 标题长度尽量控制在 %d-%d 字。
- 若信息不足，输出保守且有信息量的标题，不补充臆测内容。

# 待生成标题内容

%s`

const (
	defaultTitleMinLen = 10
	defaultTitleMaxLen = 15
)

type TitleMakerImpl struct {
	provider llm.Provider
	model    string
	llm      *chat.Gateway
}

func NewTitleMaker(gw *chat.Gateway, provider llm.Provider, model string) adapter.TitleMaker {
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
		provider = llm.Provider(opt.Provider)
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

	llmModel, err := t.llm.GetProvider(provider)
	if err != nil {
		return "", errors.Wrapf(errors.ErrParams, "get provider failed, err=%v", err)
	}

	result, err := llmModel.Generate(ctx,
		[]*einoschema.Message{
			{
				Role:    einoschema.User,
				Content: prompt,
			},
		}, chat.WithModel(model),
		chat.WithThinking(provider, false),
	)
	if err != nil {
		return "", errors.WithMessagef(err, "generate title failed, err=%v", err)
	}

	return strings.TrimSpace(result.Content), nil
}
