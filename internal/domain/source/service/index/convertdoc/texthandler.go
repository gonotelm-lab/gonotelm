package convertdoc

import (
	"context"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index/convertdoc/transformer"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index/util"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/cloudwego/eino/components/document/parser"
)

var _ Handler = (*TextHandler)(nil)

type TextHandler struct {
	impl *baseHandler
}

func NewTextHandler(c HandlerConfig) *TextHandler {
	return &TextHandler{
		impl: newBaseHandler("text-pipe", parser.TextParser{}, c),
	}
}

func (e *TextHandler) Handle(
	ctx context.Context,
	src *entity.Source,
	opts ...HandleOption,
) (*HandleResult, error) {
	textContent, err := src.GetTextContent()
	if err != nil {
		return nil, errors.Wrap(err, "get text content failed")
	}

	splitMethod := transformer.ChunkRecursiveSplitMethod
	if util.MaybeHasMarkdownHeading(textContent.Text) {
		splitMethod = transformer.ChunkMarkdownSplitMethod
	}

	docs, converted, err := e.impl.doHandle(
		ctx,
		src,
		strings.NewReader(textContent.Text),
		append([]HandleOption{}, opts...),
		nil,
		transformer.WithChunkSplitMethod(splitMethod),
	)
	if err != nil {
		return nil, errors.Wrap(err, "handle text source failed")
	}

	return &HandleResult{
		Docs:              docs,
		ParsedContent:     converted,
		ParsedContentType: entity.MimeTypeMarkdown,
	}, nil
}
