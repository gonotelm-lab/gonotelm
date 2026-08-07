package text2image

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gonotelm-lab/multimodal/image"
	"github.com/gonotelm-lab/multimodal/image/agnes"
	"github.com/gonotelm-lab/multimodal/image/dashscope"

	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
)

const defaultGenerateTimeout = 15 * time.Minute

func newText2ImageGenerator(
	provider Text2ImageProvider,
	cfg *Text2ImageConfig,
	opts ...image.ClientOption,
) (image.Generator, error) {
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

func withImageHTTPClient(opts []image.ClientOption, timeout time.Duration) []image.ClientOption {
	if timeout <= 0 {
		timeout = defaultGenerateTimeout
	}

	var base http.RoundTripper = &http.Transport{
		ForceAttemptHTTP2:     true,
		DialContext:           httpclient.DefaultDialer.DialContext,
		MaxIdleConns:          200,
		IdleConnTimeout:       120 * time.Second,
		ResponseHeaderTimeout: time.Minute,
		TLSHandshakeTimeout:   time.Minute,
	}

	builder := httpclient.NewBuilder(base)
	client := builder.WithTimeout(timeout).Build()

	newOpts := []image.ClientOption{}
	newOpts = append(newOpts, opts...)
	newOpts = append(newOpts, image.WithHTTPClient(client))

	return newOpts
}
