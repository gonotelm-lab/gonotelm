package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

const (
	csrfDefaultTokenLength = 32
	csrfDefaultHeaderName  = "X-CSRF-Token"
)

// CSRFConfig configures the double-submit cookie CSRF middleware.
type CSRFConfig struct {
	CookieName  string
	HeaderName  string
	TokenLength int
	Secure      bool
	SameSite    protocol.CookieSameSite
	MaxAge      int
	// OnError writes the rejection response; defaults to 403.
	OnError func(ctx context.Context, rc *app.RequestContext)
}

// CSRF implements the double-submit cookie pattern: a random token is issued in
// a (JS-readable) cookie on safe requests and must be echoed back in a header on
// state-changing requests. The cookie must be same-origin readable by the
// frontend; combine with SameSite cookies for defense in depth.
func CSRF(cfg CSRFConfig) app.HandlerFunc {
	if cfg.CookieName == "" {
		panic("middleware: CSRF CookieName is required")
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = csrfDefaultHeaderName
	}
	if cfg.TokenLength <= 0 {
		cfg.TokenLength = csrfDefaultTokenLength
	}

	onError := cfg.OnError
	if onError == nil {
		onError = func(_ context.Context, rc *app.RequestContext) {
			rc.AbortWithStatus(http.StatusForbidden)
		}
	}

	return func(ctx context.Context, rc *app.RequestContext) {
		safe := isSafeMethod(string(rc.Method()))

		token := string(rc.Cookie(cfg.CookieName))
		if token == "" {
			// Issue a token only on safe requests; a state-changing request
			// without a token can't be verified, so reject it.
			if !safe {
				onError(ctx, rc)
				return
			}

			newToken, err := randomToken(cfg.TokenLength)
			if err != nil {
				onError(ctx, rc)
				return
			}
			token = newToken
			rc.SetCookie(cfg.CookieName, token, cfg.MaxAge, "/", "", cfg.SameSite, cfg.Secure, false)
		}

		if !safe {
			header := string(rc.GetHeader(cfg.HeaderName))
			if subtle.ConstantTimeCompare([]byte(header), []byte(token)) != 1 {
				onError(ctx, rc)
				return
			}
		}

		rc.Next(ctx)
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
