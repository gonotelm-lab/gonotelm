package trace

import (
	"context"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// RestoreReqIdFromTrace ensures pkgcontext has a req_id: reuse existing value,
// copy from the active OTEL trace ID, or generate a new one when trace is absent.
// Used by async workers (e.g. flow) that may receive traceparent but not HTTP headers.
func RestoreReqIdFromTrace(ctx context.Context) context.Context {
	if !pkgcontext.GetReqId(ctx).IsZero() {
		return ctx
	}

	sc := oteltrace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		return pkgcontext.WithReqId(ctx, requestid.ID(sc.TraceID()))
	}

	return pkgcontext.WithReqId(ctx, requestid.Gen())
}
