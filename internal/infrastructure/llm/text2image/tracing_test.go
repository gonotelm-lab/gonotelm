package text2image

import (
	"context"
	"testing"

	pkgt2i "github.com/gonotelm-lab/multimodal/image"
	pkgt2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func newSpanExporter(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(sdktrace.NewTracerProvider())
	})
	return exporter
}

type fakeGenerator struct {
	genFn func(ctx context.Context, req *pkgt2ischema.Request, opts ...pkgt2i.Option) (*pkgt2ischema.Response, error)
}

func (f *fakeGenerator) Generate(ctx context.Context, req *pkgt2ischema.Request, opts ...pkgt2i.Option) (*pkgt2ischema.Response, error) {
	if f.genFn != nil {
		return f.genFn(ctx, req, opts...)
	}
	return &pkgt2ischema.Response{ImageURL: "http://example.com/i.png"}, nil
}

func TestTracingGeneratorSpan(t *testing.T) {
	exporter := newSpanExporter(t)
	tracer := &tracingGenerator{system: "agnes", impl: &fakeGenerator{}}

	_, err := tracer.Generate(context.Background(), &pkgt2ischema.Request{Model: "agnesi", Prompt: "cat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	sp := spans[0]
	if sp.Name != "gen_ai.text2image" {
		t.Fatalf("span name: got %q want %q", sp.Name, "gen_ai.text2image")
	}
	if sp.SpanKind != oteltrace.SpanKindClient {
		t.Fatalf("span kind: got %v want client", sp.SpanKind)
	}
	assertSpanAttr(t, sp, "gen_ai.system", "agnes")
	assertSpanAttr(t, sp, "gen_ai.operation.name", "text2image")
	assertSpanAttr(t, sp, "gen_ai.request.model", "agnesi")
	if sp.Status.Code != codes.Unset {
		t.Fatalf("status: got %v want unset", sp.Status.Code)
	}
}

func TestTracingGeneratorRecordsError(t *testing.T) {
	exporter := newSpanExporter(t)
	tracer := &tracingGenerator{system: "dashscope", impl: &fakeGenerator{
		genFn: func(ctx context.Context, req *pkgt2ischema.Request, opts ...pkgt2i.Option) (*pkgt2ischema.Response, error) {
			return nil, context.Canceled
		},
	}}

	if _, err := tracer.Generate(context.Background(), &pkgt2ischema.Request{Prompt: "cat"}); err == nil {
		t.Fatal("want error")
	}

	sp := exporter.GetSpans()[0]
	if sp.Status.Code != codes.Error {
		t.Fatalf("status: got %v want error", sp.Status.Code)
	}
	if len(sp.Events) == 0 {
		t.Fatal("want recorded error event")
	}
}

func assertSpanAttr(t *testing.T, sp tracetest.SpanStub, key, want string) {
	t.Helper()
	for _, a := range sp.Attributes {
		if string(a.Key) == key {
			if got := a.Value.AsString(); got != want {
				t.Fatalf("attr %s: got %q want %q", key, got, want)
			}
			return
		}
	}
	t.Fatalf("attr %s not found", key)
}
