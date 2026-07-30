package httpclient

import (
	"net/http"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type Builder struct {
	baseTransport http.RoundTripper

	maxRedirects int
	timeout      time.Duration
	maxRetries   int
	retryOptions []RetryOption
}

func NewBuilder(baseRoundTripper http.RoundTripper) *Builder {
	return &Builder{
		timeout:       time.Second * 30,
		maxRedirects:  10,
		maxRetries:    3,
		baseTransport: baseRoundTripper,
	}
}

func (b *Builder) WithTimeout(timeout time.Duration) *Builder {
	b.timeout = timeout
	return b
}

func (b *Builder) WithMaxRedirects(maxRedirects int) *Builder {
	b.maxRedirects = maxRedirects
	return b
}

func (b *Builder) WithRetries(times int) *Builder {
	b.maxRetries = times
	return b
}

func (b *Builder) WithRetryOptions(opts ...RetryOption) *Builder {
	b.retryOptions = append(b.retryOptions, opts...)
	return b
}

func (b *Builder) Build() *http.Client {
	baseTransport := b.baseTransport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	baseTransport = NewRetryRoundTripper(
		b.maxRetries,
		baseTransport,
		b.retryOptions...,
	)

	return &http.Client{
		Timeout: b.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= b.maxRedirects {
				return errors.ErrParams.Msgf("max redirects reached, url=%s", req.URL.String())
			}
			return nil
		},
		Transport: baseTransport,
	}
}
