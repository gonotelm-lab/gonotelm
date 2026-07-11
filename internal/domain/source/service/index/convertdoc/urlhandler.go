package convertdoc

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/cloudwego/eino/components/document/parser"
)

var _ Handler = (*UrlHandler)(nil)

type UrlHandler struct {
	impl *baseHandler
}

func NewUrlHandler(c HandlerConfig) *UrlHandler {
	return &UrlHandler{
		impl: newBaseHandler("url-pipe", parser.TextParser{}, c),
	}
}

func (e *UrlHandler) Handle(
	ctx context.Context,
	src *entity.Source,
	opts ...HandleOption,
) (*HandleResult, error) {
	urlContent, err := src.GetUrlContent()
	if err != nil {
		return nil, errors.Wrap(err, "get url content failed")
	}

	_ = urlContent

	return nil, errors.New("not implemented")
}
