package adapter

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	einoschema "github.com/cloudwego/eino/schema"
)

type ImageInterpreterImpl struct {
	modelProvider *chat.ModelProvider
	gateway       *chat.Gateway
	model         chat.Model
}

const (
	imageInterpreterSystemPrompt = `Convert the image into structured Markdown.
Be faithful. Do not invent text, numbers, or objects.
Transcribe all visible text in reading order, in the original language.
Output the body only.

Required:
## Summary
Image type, topic, and key facts. 2-4 sentences.

Then add a ## heading ONLY when that content exists in the image.
If a category is absent: omit the heading entirely. Do not write "none", "n/a", or a placeholder.

Allowed optional headings and what they contain:
- ## Text — full OCR; preserve headings, lists, etc
- ## Tables — markdown tables
- ## Charts — type, axes, units, legend, readable values or relative trends
- ## Diagrams — nodes, edges, grouping; keep original labels
- ## Formulas and code — LaTeX for formulas; fenced code with original indentation
- ## Visual details — only what text does not cover; no aesthetic commentary`
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
	t, perr := url.Parse(input)
	if perr != nil || t == nil {
		return "", errors.ErrParams.Msg("invalid image url")
	}

	if t.Scheme != "http" && t.Scheme != "https" && t.Scheme != "data" {
		return "", errors.ErrParams.Msg("invalid image url")
	}

	lang := pkgcontext.GetLang(ctx)
	if lang == "" {
		lang = "zh-CN"
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
					Type: einoschema.ChatMessagePartTypeText,
					Text: fmt.Sprintf("Please interpret the image in language %s", lang),
				},
				{
					Type: einoschema.ChatMessagePartTypeImageURL,
					Image: &einoschema.MessageInputImage{
						MessagePartCommon: einoschema.MessagePartCommon{
							URL: &input,
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
		chat.WithThinking(i.modelProvider.Provider(), false), // we don't need thinking for this task
	)
	if err != nil {
		return "", errors.WithMessagef(err, "failed to generate image interpreter: %v", err)
	}
	return result.Content, nil
}

func (i *ImageInterpreterImpl) InterpretBytes(ctx context.Context, bytes []byte) (string, error) {
	mime := mimetype.Detect(bytes)
	if !strings.HasPrefix(mime.String(), "image/") {
		return "", errors.ErrParams.Msg("input bytes is not an image")
	}
	data := buildImageBytesBase64(bytes, mime.String())
	return i.Interpret(ctx, data)
}

func (i *ImageInterpreterImpl) InterpretReader(ctx context.Context, reader io.Reader) (string, error) {
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return "", errors.WithMessagef(err, "failed to read image reader: %v", err)
	}
	return i.InterpretBytes(ctx, bytes)
}

func buildImageBytesBase64(src []byte, mimetype string) string {
	encodedLen := base64.StdEncoding.EncodedLen(len(src))
	buf := make([]byte, 13+len(mimetype)+encodedLen)
	n := copy(buf, "data:") // 5
	n += copy(buf[n:], mimetype)
	n += copy(buf[n:], ";base64,") // 8
	base64.StdEncoding.Encode(buf[n:], src)
	return pkgstring.FromBytes(buf)
}
