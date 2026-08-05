package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func newTestEngine() (*route.Engine, *tracetest.InMemoryExporter) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)

	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(Tracing("test-server"))
	return engine, exporter
}

func spanAttrs(t *testing.T, exporter *tracetest.InMemoryExporter) map[attribute.Key]attribute.Value {
	t.Helper()
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	m := make(map[attribute.Key]attribute.Value, len(spans[0].Attributes))
	for _, a := range spans[0].Attributes {
		m[a.Key] = a.Value
	}
	return m
}

func TestTracingEmitsNewSemconv(t *testing.T) {
	engine, exporter := newTestEngine()
	engine.GET("/api/users/:id", func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusOK)
	})

	w := ut.PerformRequest(engine, "GET", "http://example.com/api/users/42?x=1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d", w.Code)
	}

	attrs := spanAttrs(t, exporter)

	want := map[attribute.Key]string{
		"http.request.method": "GET",
		"url.path":            "/api/users/42",
		"url.scheme":          "http",
		"http.route":          "/api/users/:id",
		"server.address":      "example.com",
	}
	for k, wantV := range want {
		if got := attrs[k]; got.AsString() != wantV {
			t.Errorf("attribute %s: got %q want %q", k, got.AsString(), wantV)
		}
	}
	if got := attrs["http.response.status_code"].AsInt64(); got != 200 {
		t.Errorf("http.response.status_code: got %d want 200", got)
	}

	// old attributes must not be emitted
	for _, old := range []string{"http.method", "http.status_code", "http.scheme", "http.target", "http.url", "http.server_name"} {
		if _, ok := attrs[attribute.Key(old)]; ok {
			t.Errorf("old attribute %s must not be emitted", old)
		}
	}

	// status: 2xx → Unset
	if got := exporter.GetSpans()[0].Status.Code; got != codes.Unset {
		t.Errorf("span status: got %v want unset", got)
	}
}

func TestTracingOriginFormServerAddress(t *testing.T) {
	engine, exporter := newTestEngine()
	engine.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusOK)
	})

	// origin-form request: relative URL, host comes from the Host header
	w := ut.PerformRequest(engine, "GET", "/ping", nil,
		ut.Header{Key: "Host", Value: "gateway.example.com"})
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d", w.Code)
	}

	attrs := spanAttrs(t, exporter)
	if got := attrs["server.address"].AsString(); got != "gateway.example.com" {
		t.Errorf("server.address: got %q want %q", got, "gateway.example.com")
	}
	if got := attrs["url.scheme"].AsString(); got != "http" {
		t.Errorf("url.scheme: got %q want %q", got, "http")
	}
}

func TestTracingStatus5xx(t *testing.T) {
	engine, exporter := newTestEngine()
	engine.GET("/err", func(ctx context.Context, c *app.RequestContext) {
		c.SetStatusCode(http.StatusInternalServerError)
	})

	ut.PerformRequest(engine, "GET", "/err", nil)

	attrs := spanAttrs(t, exporter)
	if got := attrs["error.type"].AsString(); got != "500" {
		t.Errorf("error.type: got %q want %q", got, "500")
	}
	if got := exporter.GetSpans()[0].Status.Code; got != codes.Error {
		t.Errorf("span status: got %v want error", got)
	}
}
