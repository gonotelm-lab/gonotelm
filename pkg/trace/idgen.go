package trace

import (
	"context"
	"encoding/binary"
	"math/rand/v2"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type IDGenerator struct{}

var _ sdktrace.IDGenerator = &IDGenerator{}

func putSpanID() oteltrace.SpanID {
	sid := oteltrace.SpanID{}
	for {
		binary.NativeEndian.PutUint64(sid[:], rand.Uint64())
		if sid.IsValid() {
			break
		}
	}

	return sid
}

func putTraceID() oteltrace.TraceID {
	tid := oteltrace.TraceID{}
	for {
		binary.NativeEndian.PutUint64(tid[:8], rand.Uint64())
		binary.NativeEndian.PutUint64(tid[8:], rand.Uint64())
		if tid.IsValid() {
			break
		}
	}

	return tid
}

// NewSpanID returns a non-zero span ID from a randomly-chosen sequence.
func (*IDGenerator) NewSpanID(ctx context.Context, traceID oteltrace.TraceID) oteltrace.SpanID {
	return putSpanID()
}

// NewIDs returns a non-zero trace ID and a non-zero span ID from a
// randomly-chosen sequence.
func (t *IDGenerator) NewIDs(ctx context.Context) (oteltrace.TraceID, oteltrace.SpanID) {
	reqId := pkgcontext.GetReqId(ctx)
	if reqId.IsZero() {
		return t.newIDs(ctx)
	}

	return oteltrace.TraceID(reqId), putSpanID()
}

func (*IDGenerator) newIDs(context.Context) (oteltrace.TraceID, oteltrace.SpanID) {
	tid := putTraceID()
	sid := putSpanID()

	return tid, sid
}

type randomIDGenerator struct{}

var _ sdktrace.IDGenerator = &randomIDGenerator{}

// NewSpanID returns a non-zero span ID from a randomly-chosen sequence.
func (*randomIDGenerator) NewSpanID(context.Context, oteltrace.TraceID) oteltrace.SpanID {
	sid := oteltrace.SpanID{}
	for {
		binary.NativeEndian.PutUint64(sid[:], rand.Uint64())
		if sid.IsValid() {
			break
		}
	}
	return sid
}

// NewIDs returns a non-zero trace ID and a non-zero span ID from a
// randomly-chosen sequence.
func (*randomIDGenerator) NewIDs(context.Context) (oteltrace.TraceID, oteltrace.SpanID) {
	tid := oteltrace.TraceID{}
	sid := oteltrace.SpanID{}
	for {
		binary.NativeEndian.PutUint64(tid[:8], rand.Uint64())
		binary.NativeEndian.PutUint64(tid[8:], rand.Uint64())
		if tid.IsValid() {
			break
		}
	}
	for {
		binary.NativeEndian.PutUint64(sid[:], rand.Uint64())
		if sid.IsValid() {
			break
		}
	}
	return tid, sid
}
