package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

func newNoStoreTestEngine() *route.Engine {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(NoStore())

	engine.GET("/ok", func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusOK)
	})
	engine.GET("/redirect", func(ctx context.Context, c *app.RequestContext) {
		c.Redirect(http.StatusFound, []byte("/login"))
	})
	engine.GET("/reject", func(ctx context.Context, c *app.RequestContext) {
		c.AbortWithStatus(http.StatusForbidden)
	})

	return engine
}

func assertNoStore(t *testing.T, path string, wantStatus int) {
	t.Helper()

	w := ut.PerformRequest(newNoStoreTestEngine(), "GET", path, nil)
	if w.Code != wantStatus {
		t.Fatalf("%s status: got %d want %d", path, w.Code, wantStatus)
	}
	if got := w.Header().Get(headerCacheControl); got != cacheControlNoStore {
		t.Fatalf("%s Cache-Control: got %q want %q", path, got, cacheControlNoStore)
	}
}

func TestNoStoreSetsCacheControl(t *testing.T) {
	assertNoStore(t, "/ok", http.StatusOK)
}

// 登录与回调大量是 302，缓存头不能只覆盖 200
func TestNoStoreSurvivesRedirect(t *testing.T) {
	assertNoStore(t, "/redirect", http.StatusFound)
}

// 被 abort 的响应也要带上，不能只在 200 这条路径上生效
func TestNoStoreSurvivesAbort(t *testing.T) {
	assertNoStore(t, "/reject", http.StatusForbidden)
}
