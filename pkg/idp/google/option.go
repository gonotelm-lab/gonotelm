package google

import "net/http"

type Option func(*options)

type options struct {
	httpClient   *http.Client
	discoveryURL string
	userInfoURL  string
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

// WithDiscoveryURL overrides the OpenID Connect Discovery document URL.
func WithDiscoveryURL(discoveryURL string) Option {
	return func(o *options) {
		if discoveryURL != "" {
			o.discoveryURL = discoveryURL
		}
	}
}

// WithUserInfoURL pins the UserInfo Endpoint and skips the Discovery document.
// Normally the endpoint is read from the `userinfo_endpoint` metadata value.
func WithUserInfoURL(userInfoURL string) Option {
	return func(o *options) {
		if userInfoURL != "" {
			o.userInfoURL = userInfoURL
		}
	}
}
