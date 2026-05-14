package convertdoc

import (
	"context"
	"strings"

	convertdoctransformer "github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc/transformer"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einoparser "github.com/cloudwego/eino/components/document/parser"
)

var _ Handler = (*TextHandler)(nil)

type TextHandler struct {
	impl *commonHandler
}

func NewTextHandler(c HandlerConfig) *TextHandler {
	return &TextHandler{
		impl: newCommonHandler("text-pipe", einoparser.TextParser{}, c),
	}
}

func (e *TextHandler) Handle(ctx context.Context, s *model.Source) (*HandleResult, error) {
	ts := model.TextSourceContent{}
	if err := decodeSourceContent(s.Content, &ts, "unmarshal text source content failed"); err != nil {
		return nil, err
	}

	docs, err := e.impl.doHandle(
		ctx,
		s,
		strings.NewReader(ts.Text),
		nil,
		convertdoctransformer.WithChunkSplitMethod(convertdoctransformer.ChunkRecursiveSplitMethod),
	)
	if err != nil {
		return nil, errors.Wrap(err, "handle text source failed")
	}

	return &HandleResult{Docs: docs}, nil
}
