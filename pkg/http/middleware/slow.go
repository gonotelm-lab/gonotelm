package middleware

import (
	"context"
	"time"

	pkghttp "github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
)

func SlowRequestThreshold(threshold time.Duration) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Set(pkghttp.RequestContextSlowLogThreshold, threshold)
	}
}

func SlowRequestSkip(skip bool) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Set(pkghttp.RequestContextSlowLogSkip, skip)
	}
}
