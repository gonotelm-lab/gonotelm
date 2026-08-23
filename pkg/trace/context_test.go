package trace

import (
	"context"
	"testing"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"

	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestRestoreReqIdFromTrace(t *testing.T) {
	t.Parallel()

	traceID := oteltrace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := oteltrace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	ctx := oteltrace.ContextWithSpanContext(context.Background(), oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	}))

	got := RestoreReqIdFromTrace(ctx)
	if pkgcontext.GetReqId(got) != requestid.ID(traceID) {
		t.Fatalf("req_id mismatch, got=%s want=%s", pkgcontext.GetReqId(got), requestid.ID(traceID))
	}

	existing := requestid.Gen()
	ctxWithReq := pkgcontext.WithReqId(ctx, existing)
	if RestoreReqIdFromTrace(ctxWithReq) != ctxWithReq {
		t.Fatal("expected existing req_id to be preserved")
	}

	gotNoTrace := RestoreReqIdFromTrace(context.Background())
	if pkgcontext.GetReqId(gotNoTrace).IsZero() {
		t.Fatal("expected generated req_id when trace is absent")
	}
}
