package kafka

import (
	"context"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"
	"github.com/gonotelm-lab/gonotelm/pkg/ulid"
	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	pkgpropagation "github.com/gonotelm-lab/gonotelm/pkg/trace/propagation"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestMessageHeadersRoundTrip(t *testing.T) {
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

	// 模拟上游请求上下文：server span + req id + user id
	reqId := requestid.Gen()
	userId := ulid.MustParseString("01hf7yat00vtpvxvyaztxbw001")
	ctx, span := otel.Tracer("test").Start(context.Background(), "server-span",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer))
	defer span.End()
	ctx = pkgcontext.WithReqId(ctx, reqId)
	ctx = pkgcontext.WithUserId(ctx, userId)

	// 生产端：注入
	req := &mq.ProducerSendRequest{
		Topic: "test-topic",
		Key:   []byte("k1"),
		Value: []byte("v1"),
		Headers: []mq.MessageHeader{
			{Key: "custom", Value: []byte("x")},
		},
	}
	msg := prepareMessage(req, buildMessageHeaders(ctx, req.Headers))

	// 1. 自定义 header 保留
	if got := getHeader(msg.Headers, "custom"); got != "x" {
		t.Fatalf("custom header lost: %v", msg.Headers)
	}

	// 2. traceparent 已注入
	traceparent := getHeader(msg.Headers, "traceparent")
	if len(traceparent) == 0 {
		t.Fatalf("traceparent not injected: %v", msg.Headers)
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 || parts[0] != "00" {
		t.Fatalf("invalid traceparent: %s", traceparent)
	}
	if parts[1] != span.SpanContext().TraceID().String() {
		t.Fatalf("trace id mismatch: got %s want %s", parts[1], span.SpanContext().TraceID())
	}

	// 3. req id / user id 已注入
	if got := getHeader(msg.Headers, requestid.HeaderKey); got != reqId.String() {
		t.Fatalf("req id header mismatch: got %s want %s", got, reqId)
	}
	if got := getHeader(msg.Headers, userIdHeaderKey); got != userId.String() {
		t.Fatalf("user id header mismatch: got %s want %s", got, userId)
	}

	// 4. 消费端：还原到 ctx
	restoredCtx := restoreRequestContext(context.Background(), msg.Headers)
	if got := pkgcontext.GetReqId(restoredCtx); got != reqId {
		t.Fatalf("restored req id mismatch: got %s want %s", got, reqId)
	}
	if got := pkgcontext.GetUserId(restoredCtx); got != userId {
		t.Fatalf("restored user id mismatch: got %s want %s", got, userId)
	}

	// 5. 消费端 trace 上下文可提取回同一 trace
	carrier := pkgpropagation.NewKafkaHeaderCarrier(msg.Headers)
	extractedCtx := pkgtrace.GetTextMapPropagator().Extract(context.Background(), carrier)
	extracted := oteltrace.SpanContextFromContext(extractedCtx)
	if extracted.TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("extract round trip mismatch: got %s want %s",
			extracted.TraceID(), span.SpanContext().TraceID())
	}
	if extracted.SpanID() != span.SpanContext().SpanID() {
		t.Fatalf("extract span id mismatch: got %s want %s",
			extracted.SpanID(), span.SpanContext().SpanID())
	}
}

func TestRestoreRequestContextInvalid(t *testing.T) {
	// 非法 req id / user id 不 panic，跳过还原
	ctx := restoreRequestContext(context.Background(), []kafka.Header{
		{Key: requestid.HeaderKey, Value: []byte("not-a-uuid")},
		{Key: userIdHeaderKey, Value: []byte("not-a-ulid")},
	})
	if got := pkgcontext.GetReqId(ctx); !got.IsZero() {
		t.Fatalf("invalid req id should not be restored: %s", got)
	}
	if got := pkgcontext.GetUserId(ctx); !got.IsZero() {
		t.Fatalf("invalid user id should not be restored: %s", got)
	}
}

func getHeader(headers []kafka.Header, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func TestProducerSpanSemconv(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(sdktrace.NewTracerProvider())
	})

	ctx := context.Background()
	tracer := otel.Tracer("test")
	_, span := tracer.Start(ctx, "server-span", oteltrace.WithSpanKind(oteltrace.SpanKindServer))
	ctx = oteltrace.ContextWithSpan(ctx, span)
	defer span.End()

	producer := &Producer{w: &fakeWriter{}}
	req := &mq.ProducerSendRequest{
		Topic: "orders",
		Key:   []byte("key-1"),
		Value: []byte("v1"),
	}
	if err := producer.Send(ctx, req); err != nil {
		t.Fatalf("Send() failed: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	s := spans[0]
	if s.Name != "send orders" {
		t.Errorf("span name: got %q want %q", s.Name, "send orders")
	}
	want := map[string]string{
		"messaging.system":            "kafka",
		"messaging.operation.name":    "send",
		"messaging.operation.type":    "send",
		"messaging.destination.name":  "orders",
		"messaging.kafka.message.key": "key-1",
	}
	for k, wantV := range want {
		found := false
		for _, a := range s.Attributes {
			if string(a.Key) == k {
				if a.Value.AsString() != wantV {
					t.Errorf("attribute %s: got %q want %q", k, a.Value.AsString(), wantV)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("attribute %s not found", k)
		}
	}
}

type fakeWriter struct{}

func (w *fakeWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	return nil
}

func (w *fakeWriter) Close() error { return nil }
