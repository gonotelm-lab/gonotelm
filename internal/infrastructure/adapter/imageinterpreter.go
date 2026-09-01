package adapter

import (
	"context"
	"fmt"
	"net/url"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einoschema "github.com/cloudwego/eino/schema"
)

type ImageInterpreterImpl struct {
	modelProvider *chat.ModelProvider
	gateway       *chat.Gateway
	model         chat.Model
}

const (
	imageInterpreterSystemPrompt = `Describe the following image in detail.`
)

var _ adapter.ImageInterpreter = (*ImageInterpreterImpl)(nil)

func NewImageInterpreter(gateway *chat.Gateway, provider chat.Provider, model string) (*ImageInterpreterImpl, error) {
	modelProvider, err := gateway.GetModelProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get model provider: %w", err)
	}

	llmm, ok := modelProvider.Model(model)
	if !ok {
		return nil, fmt.Errorf("model %s not supported by provider %s", model, provider)
	}

	if !llmm.Modalities.SupportImageInput() {
		return nil, fmt.Errorf("model %s does not support image input", model)
	}

	return &ImageInterpreterImpl{
		modelProvider: modelProvider,
		gateway:       gateway,
		model:         llmm,
	}, nil
}

func (i *ImageInterpreterImpl) Interpret(ctx context.Context, input string) (string, error) {
	var (
		urlStr     *string
		base64Data *string
	)
	_, perr := url.Parse(input)
	if perr == nil {
		urlStr = &input
	} else {
		base64Data = &input
	}

	tccm := i.modelProvider.ToolCallingChatModel()
	msgs := []*einoschema.Message{
		{
			Role:    einoschema.System,
			Content: imageInterpreterSystemPrompt,
		},
		{
			Role: einoschema.User,
			UserInputMultiContent: []einoschema.MessageInputPart{
				{
					Type: einoschema.ChatMessagePartTypeImageURL,
					Image: &einoschema.MessageInputImage{
						MessagePartCommon: einoschema.MessagePartCommon{
							URL:        urlStr,
							Base64Data: base64Data,
						},
					},
				},
			},
		},
	}

	result, err := tccm.Generate(
		ctx,
		msgs,
		chat.WithModel(i.model.Name),
		chat.WithThinking(i.modelProvider.Provider(), false),
	)
	if err != nil {
		return "", errors.WithMessagef(err, "failed to generate image interpreter: %v", err)
	}
	return result.Content, nil
}

func (i *ImageInterpreterImpl) InterpretBytes(ctx context.Context, bytes []byte) (string, error) {
	return "", nil
}
