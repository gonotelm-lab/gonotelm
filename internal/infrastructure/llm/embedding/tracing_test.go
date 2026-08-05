package embedding

import (
	"context"
	"testing"

	einoembed "github.com/cloudwego/eino/components/embedding"
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

type fakeEmbedder struct {
	embedFn func(ctx context.Context, texts []string, opts ...einoembed.Option) ([][]float64, error)
}

func (f *fakeEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...einoembed.Option) ([][]float64, error) {
	if f.embedFn != nil {
		return f.embedFn(ctx, texts, opts...)
	}
	return [][]float64{{1, 2, 3}}, nil
}

func TestTracingEmbedderSpan(t *testing.T) {
	exporter := newSpanExporter(t)
	tracer := &tracingEmbedder{system: "dashscope", impl: &fakeEmbedder{}}

	model := "text-embedding-v3"
	_, err := tracer.EmbedStrings(context.Background(), []string{"hello"},
		einoembed.WithModel(model))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	sp := spans[0]
	if sp.Name != "gen_ai.embedding" {
		t.Fatalf("span name: got %q want %q", sp.Name, "gen_ai.embedding")
	}
	if sp.SpanKind != oteltrace.SpanKindClient {
		t.Fatalf("span kind: got %v want client", sp.SpanKind)
	}
	assertSpanAttr(t, sp, "gen_ai.system", "dashscope")
	assertSpanAttr(t, sp, "gen_ai.operation.name", "embedding")
	assertSpanAttr(t, sp, "gen_ai.request.model", model)
	if sp.Status.Code != codes.Unset {
		t.Fatalf("status: got %v want unset", sp.Status.Code)
	}
}

func TestTracingEmbedderRecordsError(t *testing.T) {
	exporter := newSpanExporter(t)
	tracer := &tracingEmbedder{system: "openai", impl: &fakeEmbedder{
		embedFn: func(ctx context.Context, texts []string, opts ...einoembed.Option) ([][]float64, error) {
			return nil, context.DeadlineExceeded
		},
	}}

	if _, err := tracer.EmbedStrings(context.Background(), []string{"hello"}); err == nil {
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
