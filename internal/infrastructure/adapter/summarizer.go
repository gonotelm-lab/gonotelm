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

const summarizePromptTemplate = `
你是一个文本摘要助手，擅长从复杂的文本中提取出核心和关键细节。请对输入文本生成高信息密度的短摘要。

# 任务

- 提炼主线主题、关键结论，以及关键事实（如实体、数字、时间、范围等，若原文存在）。
- 去除铺垫、重复表述、次要细节和与结论弱相关的信息。
- 保持忠实，不得编造、延伸推断或引入原文未出现的事实。

# 输出约束

- 仅输出摘要正文，不要标题、不要分点、不要解释、不要任何前后缀。
- 使用 3-5 句话，长度尽量控制在 %d-%d 字。
- 若信息不足以支撑完整结论，保守概括已有信息，不补充臆测内容。

# 待摘要内容

%s`

const (
	defaultMinWord = 60
	defaultMaxWord = 150
)

type SummarizerImpl struct {
	provider llm.Provider
	llm      *chat.Gateway
}

func NewSummarizer(llm *chat.Gateway) adapter.Summarizer {
	return &SummarizerImpl{
		llm: llm,
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
	if opt.Prompt != "" {
		provider = llm.Provider(opt.Provider)
	}

	var prompt string
	if opt.Prompt != "" {
		prompt = opt.Prompt
	} else {
		prompt = fmt.Sprintf(summarizePromptTemplate, opt.MinWord, opt.MaxWord, text)
	}

	model, err := s.llm.GetProvider(provider)
	if err != nil {
		return "", errors.Wrapf(errors.ErrParams, "get provider failed, err=%v", err)
	}

	result, err := model.Generate(ctx, []*einoschema.Message{
		{
			Role:    einoschema.User,
			Content: prompt,
		},
	}, llm.WithModel(opt.Model))
	if err != nil {
		return "", errors.WithMessagef(err, "generate summary failed, err=%v", err)
	}

	return strings.TrimSpace(result.Content), nil
}
