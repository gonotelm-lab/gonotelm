package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

const (
	corsHeaderOrigin        = "Origin"
	corsHeaderRequestMethod = "Access-Control-Request-Method"

	corsHeaderAllowOrigin  = "Access-Control-Allow-Origin"
	corsHeaderAllowCreds   = "Access-Control-Allow-Credentials"
	corsHeaderAllowMethods = "Access-Control-Allow-Methods"
	corsHeaderAllowHeaders = "Access-Control-Allow-Headers"
	corsHeaderMaxAge       = "Access-Control-Max-Age"
	corsHeaderVary         = "Vary"

	// Preflight cache duration; browsers cap this themselves (Chrome caps at 2h).
	corsMaxAge = 2 * time.Hour
)

var (
	corsAllowMethods = strings.Join([]string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}, ", ")

	corsDefaultAllowHeaders = []string{
		"Content-Type",
		"Authorization",
		"Accept",
		"X-Request-Id",
	}
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	AllowOrigins []string
	// AllowHeaders 追加到默认允许的请求头之后
	AllowHeaders []string
}

// CORS handles cross-origin requests; the allowed origins are passed in from
// config (see [cors] allowOrigins).
//
// The frontend needs to send cookies, so we don't use the wildcard "*" here.
// Instead we echo the matched Origin and set Access-Control-Allow-Credentials:
// true -- the spec requires the two to appear together, since a credentialed
// cross-origin request is rejected outright by the browser when Allow-Origin
// is "*".
func CORS(cfg CORSConfig) app.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowOrigins))
	allowAll := false
	for _, origin := range cfg.AllowOrigins {
		origin = strings.TrimSpace(origin)
		switch {
		case origin == "":
			continue
		case origin == "*":
			allowAll = true
		default:
			allowed[origin] = struct{}{}
		}
	}

	allowHeaders := corsDefaultAllowHeaders
	if len(cfg.AllowHeaders) > 0 {
		allowHeaders = append(append([]string{}, corsDefaultAllowHeaders...), cfg.AllowHeaders...)
	}
	allowHeadersValue := strings.Join(allowHeaders, ", ")

	return func(ctx context.Context, rc *app.RequestContext) {
		origin := string(rc.Request.Header.Peek(corsHeaderOrigin))
		if origin == "" {
			// Non-CORS requests (curl, service-to-service calls) pass through.
			rc.Next(ctx)
			return
		}

		if _, ok := allowed[origin]; !ok && !allowAll {
			// Origin not in the allowlist: emit no CORS response headers and let
			// the browser block the response. There is no point continuing a
			// preflight, so return 403 directly, making it easy to spot a missing
			// origin in logs/curl.
			if isCorsPreflight(rc) {
				rc.AbortWithStatus(http.StatusForbidden)
				return
			}
			rc.Next(ctx)
			return
		}

		header := &rc.Response.Header
		header.Set(corsHeaderAllowOrigin, origin)
		header.Set(corsHeaderAllowCreds, "true")
		// The response varies by Origin, so a cache can't serve one site's
		// response to another.
		header.Add(corsHeaderVary, corsHeaderOrigin)

		if isCorsPreflight(rc) {
			header.Set(corsHeaderAllowMethods, corsAllowMethods)
			header.Set(corsHeaderAllowHeaders, allowHeadersValue)
			header.Set(corsHeaderMaxAge, strconv.Itoa(int(corsMaxAge.Seconds())))
			rc.AbortWithStatus(http.StatusNoContent)
			return
		}

		rc.Next(ctx)
	}
}

// isCorsPreflight reports whether this is a CORS preflight:
// OPTIONS + Access-Control-Request-Method.
func isCorsPreflight(rc *app.RequestContext) bool {
	if string(rc.Request.Header.Method()) != http.MethodOptions {
		return false
	}
	return len(rc.Request.Header.Peek(corsHeaderRequestMethod)) > 0
}
