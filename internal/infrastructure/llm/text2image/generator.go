package text2image

import (
	"fmt"

	pkgt2i "github.com/gonotelm-lab/multimodal/image"
	"github.com/gonotelm-lab/multimodal/image/agnes"
	"github.com/gonotelm-lab/multimodal/image/dashscope"
)

func newText2ImageGenerator(
	provider Text2ImageProvider,
	cfg *Text2ImageConfig,
	opts ...pkgt2i.ClientOption,
) (pkgt2i.Generator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("text2image config must not be nil")
	}

	switch provider {
	case Text2ImageDashScope:
		return dashscope.New(cfg.DashScope, opts...)
	case Text2ImageAgnes:
		return agnes.New(cfg.Agnes, opts...)
	default:
		return nil, fmt.Errorf("text2image provider %q is not supported", provider)
	}
}
