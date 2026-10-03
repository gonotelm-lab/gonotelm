package middleware

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

const testOrigin = "http://127.0.0.1:5173"

// 只注册 GET，OPTIONS 故意不注册：验证全局中间件对 404/405 也生效
func newCORSTestEngine(allowOrigins []string) *route.Engine {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(CORS(CORSConfig{AllowOrigins: allowOrigins}))
	engine.GET("/api/v1/ping", func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusOK)
	})
	return engine
}

func preflightHeaders() []ut.Header {
	return []ut.Header{
		{Key: "Origin", Value: testOrigin},
		{Key: "Access-Control-Request-Method", Value: "GET"},
	}
}

func TestCORSAllowHeadersConfig(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(CORS(CORSConfig{
		AllowOrigins: []string{testOrigin},
		AllowHeaders: []string{"X-Custom", csrfDefaultHeaderName},
	}))
	engine.GET("/api/v1/ping", func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusOK)
	})

	w := ut.PerformRequest(engine, "OPTIONS", "/api/v1/ping", nil, preflightHeaders()...)

	got := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(got, "X-Custom") {
		t.Errorf("Allow-Headers: got %q want it to contain X-Custom", got)
	}
	if !strings.Contains(got, csrfDefaultHeaderName) {
		t.Errorf("Allow-Headers: got %q want it to keep default %s", got, csrfDefaultHeaderName)
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	engine := newCORSTestEngine([]string{testOrigin})

	w := ut.PerformRequest(engine, "GET", "/api/v1/ping", nil,
		ut.Header{Key: "Origin", Value: testOrigin})

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Errorf("Allow-Origin: got %q want %q", got, testOrigin)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials: got %q want %q", got, "true")
	}
	if got := w.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary: got %q want it to contain Origin", got)
	}
}

// OPTIONS 没有注册路由，仍然要能拿到正确的预检响应
func TestCORSPreflightOnUnregisteredMethod(t *testing.T) {
	engine := newCORSTestEngine([]string{testOrigin})

	w := ut.PerformRequest(engine, "OPTIONS", "/api/v1/ping", nil, preflightHeaders()...)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusNoContent)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Errorf("Allow-Origin: got %q want %q", got, testOrigin)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Allow-Methods: got %q want it to contain POST", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Errorf("Allow-Headers: got %q want it to contain Content-Type", got)
	}
	if got := w.Header().Get("Access-Control-Max-Age"); got == "" {
		t.Error("Max-Age: expected non-empty")
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	engine := newCORSTestEngine([]string{testOrigin})

	w := ut.PerformRequest(engine, "GET", "/api/v1/ping", nil,
		ut.Header{Key: "Origin", Value: "http://evil.example.com"})

	// 业务照常执行，但不下发 CORS 头，由浏览器拦截
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin: got %q want empty", got)
	}
}

func TestCORSDisallowedOriginPreflight(t *testing.T) {
	engine := newCORSTestEngine([]string{testOrigin})

	w := ut.PerformRequest(engine, "OPTIONS", "/api/v1/ping", nil,
		ut.Header{Key: "Origin", Value: "http://evil.example.com"},
		ut.Header{Key: "Access-Control-Request-Method", Value: "GET"})

	if w.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusForbidden)
	}
}

func TestCORSWithoutOrigin(t *testing.T) {
	engine := newCORSTestEngine([]string{testOrigin})

	w := ut.PerformRequest(engine, "GET", "/api/v1/ping", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin: got %q want empty", got)
	}
}

// 配成 * 时也回显具体 Origin：带凭证的请求不允许用通配符
func TestCORSAllowAllEchoesOrigin(t *testing.T) {
	engine := newCORSTestEngine([]string{"*"})

	w := ut.PerformRequest(engine, "GET", "/api/v1/ping", nil,
		ut.Header{Key: "Origin", Value: testOrigin})

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Errorf("Allow-Origin: got %q want %q", got, testOrigin)
	}
}
