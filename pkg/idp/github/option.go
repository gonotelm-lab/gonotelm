package github

import "net/http"

type Option func(*options)

type options struct {
	httpClient *http.Client
}

func buildOptions(opts ...Option) *options {
	o := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		if client != nil {
			o.httpClient = client
		}
	}
}
