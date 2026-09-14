package httpclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestBuildClientInjectsTraceparent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	})

	var captured http.Header
	base := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		captured = req.Header.Clone()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})

	client := NewBuilder(base).WithRetries(0).Build()

	// 模拟上游已有一个 server span
	ctx, span := otel.Tracer("test").Start(
		context.Background(), "server-span", oteltrace.WithSpanKind(oteltrace.SpanKindServer),
	)
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.llm.example.com/v1/chat/completions", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	// otelhttp 的 client span 在 response body 关闭/读完时才会结束
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	traceparent := captured.Get("Traceparent")
	if len(traceparent) == 0 {
		t.Fatal("traceparent not injected into outbound request")
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 || parts[0] != "00" {
		t.Fatalf("invalid traceparent: %s", traceparent)
	}
	if parts[1] != span.SpanContext().TraceID().String() {
		t.Fatalf("trace id mismatch: got %s want %s", parts[1], span.SpanContext().TraceID())
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 client span, got %d", len(spans))
	}
	if spans[0].SpanKind != oteltrace.SpanKindClient {
		t.Fatalf("unexpected span kind: %s", spans[0].SpanKind)
	}
	if spans[0].Parent.TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("client span not child of server span")
	}
}

func TestBuilderResponseHeaderTimeout(t *testing.T) {
	// 未配置：沿用共享 DefaultTransport
	if got := NewBuilder(nil).defaultTransport(); got != DefaultTransport {
		t.Fatalf("expected shared DefaultTransport when unset, got %T", got)
	}

	// 配置为默认值：不克隆
	if got := NewBuilder(nil).WithResponseHeaderTimeout(DefaultResponseHeaderTimeout).defaultTransport(); got != DefaultTransport {
		t.Fatalf("expected shared DefaultTransport when set to default, got %T", got)
	}

	// 显式覆盖：克隆并设置 ResponseHeaderTimeout，且不改动全局
	tr, ok := NewBuilder(nil).WithResponseHeaderTimeout(30 * time.Second).defaultTransport().(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport")
	}
	if tr.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("got %s want 30s", tr.ResponseHeaderTimeout)
	}
	if DefaultTransport.ResponseHeaderTimeout != DefaultResponseHeaderTimeout {
		t.Fatalf("global DefaultTransport mutated: %s", DefaultTransport.ResponseHeaderTimeout)
	}
}
