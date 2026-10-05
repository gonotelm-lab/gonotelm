package middleware

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"
)

const (
	testCSRFCookie = "gnlm_csrf"
	testCSRFHeader = "X-CSRF-Token"
)

func newCSRFTestEngine() *route.Engine {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(CSRF(CSRFConfig{
		CookieName: testCSRFCookie,
		HeaderName: testCSRFHeader,
		SameSite:   protocol.CookieSameSiteLaxMode,
	}))
	ok := func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusOK)
	}
	engine.GET("/ping", ok)
	engine.POST("/action", ok)
	return engine
}

func csrfTokenFromSetCookie(w *ut.ResponseRecorder) string {
	var token string
	w.Header().VisitAll(func(key, value []byte) {
		if !strings.EqualFold(string(key), "Set-Cookie") {
			return
		}
		raw := string(value)
		if !strings.HasPrefix(raw, testCSRFCookie+"=") {
			return
		}
		pair := strings.SplitN(raw, ";", 2)[0]
		token = strings.TrimPrefix(pair, testCSRFCookie+"=")
	})
	return token
}

func TestCSRFIssuesTokenOnSafeRequest(t *testing.T) {
	engine := newCSRFTestEngine()

	w := ut.PerformRequest(engine, "GET", "/ping", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusOK)
	}
	if csrfTokenFromSetCookie(w) == "" {
		t.Fatal("expected a csrf token cookie to be set")
	}
}

func TestCSRFRejectsUnsafeRequestWithoutToken(t *testing.T) {
	engine := newCSRFTestEngine()

	w := ut.PerformRequest(engine, "POST", "/action", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusForbidden)
	}
}

func TestCSRFRejectsMismatchedHeader(t *testing.T) {
	engine := newCSRFTestEngine()

	w := ut.PerformRequest(engine, "POST", "/action", nil,
		ut.Header{Key: "Cookie", Value: testCSRFCookie + "=token-a"},
		ut.Header{Key: testCSRFHeader, Value: "token-b"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusForbidden)
	}
}

func TestCSRFAcceptsMatchingHeader(t *testing.T) {
	engine := newCSRFTestEngine()

	token := csrfTokenFromSetCookie(ut.PerformRequest(engine, "GET", "/ping", nil))
	if token == "" {
		t.Fatal("expected a csrf token cookie to be set")
	}

	w := ut.PerformRequest(engine, "POST", "/action", nil,
		ut.Header{Key: "Cookie", Value: testCSRFCookie + "=" + token},
		ut.Header{Key: testCSRFHeader, Value: token})
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", w.Code, http.StatusOK)
	}
}
