package convertdoc

import (
	"context"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc/transformer"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einoparser "github.com/cloudwego/eino/components/document/parser"
)

var _ Handler = (*TextHandler)(nil)

type TextHandler struct {
	impl *baseHandler
}

func NewTextHandler(c HandlerConfig) *TextHandler {
	return &TextHandler{
		impl: newBaseHandler("text-pipe", einoparser.TextParser{}, c),
	}
}

func (e *TextHandler) Handle(ctx context.Context, s *model.Source) (*HandleResult, error) {
	textSource := model.TextSourceContent{}
	if err := decodeSourceContent(s.Content, &textSource, "unmarshal text source content failed"); err != nil {
		return nil, err
	}

	docs, converted, err := e.impl.doHandle(
		ctx,
		s,
		strings.NewReader(textSource.Text),
		nil,
		transformer.WithChunkSplitMethod(transformer.ChunkRecursiveSplitMethod),
	)
	if err != nil {
		return nil, errors.Wrap(err, "handle text source failed")
	}

	return &HandleResult{
		Docs:              docs,
		ParsedContent:     converted,
		ParsedContentType: markdownMimeType,
	}, nil
}
