package httpclient

import (
	"net"
	"net/http"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var (
	maxIdleConns          = 200
	idleConnTimeout       = 120 * time.Second
	expectContinueTimeout = 1 * time.Second
	responseHeaderTimeout = 10 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second

	DefaultDialer = &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 15 * time.Second,
	}
	DefaultTransport http.RoundTripper = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           DefaultDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxIdleConns,
		IdleConnTimeout:       idleConnTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
	}
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

// 整个请求的超时时间
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
		baseTransport = DefaultTransport
	}

	// 重试计数 需要放在链路追踪包住
	baseTransport = retryAttemptInjectRoundTrip(baseTransport)

	// 链路追踪
	baseTransport = otelhttp.NewTransport(baseTransport)

	// 重试
	baseTransport = NewRetryRoundTripper(
		b.maxRetries,
		baseTransport,
		b.retryOptions...,
	)

	return &http.Client{
		Timeout:       b.timeout,
		CheckRedirect: defaultRedirectCheck(b.maxRedirects),
		Transport:     baseTransport,
	}
}

func defaultRedirectCheck(maxRedirects int) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.ErrParams.Msgf("max redirects reached, url=%s", req.URL.String())
		}
		return nil
	}
}
