package text2image

import (
	"fmt"
	"time"

	pkgt2i "github.com/gonotelm-lab/multimodal/image"
	"github.com/gonotelm-lab/multimodal/image/agnes"
	"github.com/gonotelm-lab/multimodal/image/dashscope"

	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
)

const defaultGenerateTimeout = 30 * time.Minute

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
		return dashscope.New(cfg.DashScope, withImageHTTPClient(opts, cfg.DashScope.Timeout)...)
	case Text2ImageAgnes:
		return agnes.New(cfg.Agnes, withImageHTTPClient(opts, cfg.Agnes.Timeout)...)
	default:
		return nil, fmt.Errorf("text2image provider %q is not supported", provider)
	}
}

func withImageHTTPClient(opts []pkgt2i.ClientOption, timeout time.Duration) []pkgt2i.ClientOption {
	if timeout <= 0 {
		timeout = defaultGenerateTimeout
	}
	client := httpclient.NewBuilder(nil).WithTimeout(timeout).Build()
	return append(append([]pkgt2i.ClientOption(nil), opts...), pkgt2i.WithHTTPClient(client))
}
