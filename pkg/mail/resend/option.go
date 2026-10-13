package resend

import "net/http"

type options struct {
	httpClient *http.Client
	baseURL    string
}

// Option changes how New creates the Resend sender
type Option func(*options)

// WithHTTPClient sets the project-wide http client
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		o.httpClient = client
	}
}

// WithBaseURL sets the API endpoint, for tests and self-hosted proxies
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
