package convertdoc

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einoparser "github.com/cloudwego/eino/components/document/parser"
)

var _ Handler = (*UrlHandler)(nil)

type UrlHandler struct {
	impl *commonHandler
}

func NewUrlHandler(c HandlerConfig) *UrlHandler {
	return &UrlHandler{
		impl: newCommonHandler("url-pipe", einoparser.TextParser{}, c),
	}
}

func (e *UrlHandler) Handle(ctx context.Context, s *model.Source) (*HandleResult, error) {
	us := model.UrlSourceContent{}
	if err := decodeSourceContent(s.Content, &us, "unmarshal url source content failed"); err != nil {
		return nil, err
	}

	return nil, errors.New("not implemented")
}
