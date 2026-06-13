package text2image

import (
	"fmt"

	pkgt2i "github.com/gonotelm-lab/gonotelm/pkg/text2image"
	"github.com/gonotelm-lab/gonotelm/pkg/text2image/dashscope"
)

func New(
	provider Provider,
	cfg *Config,
	opts ...pkgt2i.ClientOption,
) (pkgt2i.Generator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("text2image config must not be nil")
	}

	switch provider {
	case DashScope:
		return dashscope.New(cfg.DashScope, opts...)
	default:
		return nil, fmt.Errorf("text2image provider %q is not supported", provider)
	}
}
