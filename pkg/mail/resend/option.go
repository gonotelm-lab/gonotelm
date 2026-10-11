package resend

import "net/http"

type options struct {
	httpClient *http.Client
	baseURL    string
}

// Option adjusts how the Resend sender is built.
type Option func(*options)

// WithHTTPClient injects the project-wide http client (timeout, tracing,
// transport-level retries all live there).
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		o.httpClient = client
	}
}

// WithBaseURL overrides the API endpoint; for tests and self-hosted proxies.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

func buildOptions(opts ...Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	return o
}
