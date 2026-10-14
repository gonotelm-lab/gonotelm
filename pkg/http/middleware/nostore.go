package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

const (
	headerCacheControl  = "Cache-Control"
	cacheControlNoStore = "no-store"
)

// NoStore marks the response uncacheable.
//
// Auth responses carry session identifiers and the account email, so anything a
// shared cache or the browser's back/forward cache hangs on to can show one
// account's data to whoever uses the same device next.
//
// The header is set before the handler runs, so redirect and error responses
// from further down the chain carry it too.
func NoStore() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Response.Header.Set(headerCacheControl, cacheControlNoStore)
		c.Next(ctx)
	}
}
