package trace

import (
	"context"
	"testing"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"

	oteltrace "go.opentelemetry.io/otel/trace"
)

var (
	benchCustom = IDGenerator{}
	benchOtel   = randomIDGenerator{}

	benchSinkTID oteltrace.TraceID
	benchSinkSID oteltrace.SpanID
)

func BenchmarkCustomNewIDs(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchSinkTID, benchSinkSID = benchCustom.NewIDs(ctx)
	}
}

func BenchmarkCustomNewIDsWithReqID(b *testing.B) {
	ctx := pkgcontext.WithReqId(context.Background(), requestid.Gen())
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchSinkTID, benchSinkSID = benchCustom.NewIDs(ctx)
	}
}

func BenchmarkCustomNewSpanID(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchSinkSID = benchCustom.NewSpanID(context.Background(), benchSinkTID)
	}
}

func BenchmarkOtelNewIDs(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchSinkTID, benchSinkSID = benchOtel.NewIDs(ctx)
	}
}

func BenchmarkOtelNewSpanID(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchSinkSID = benchOtel.NewSpanID(context.Background(), benchSinkTID)
	}
}

func BenchmarkCustomNewIDsParallel(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var tid oteltrace.TraceID
		var sid oteltrace.SpanID
		for pb.Next() {
			tid, sid = benchCustom.NewIDs(ctx)
		}
		benchSinkTID, benchSinkSID = tid, sid
	})
}

func BenchmarkCustomNewIDsWithReqIDParallel(b *testing.B) {
	ctx := pkgcontext.WithReqId(context.Background(), requestid.Gen())
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var tid oteltrace.TraceID
		var sid oteltrace.SpanID
		for pb.Next() {
			tid, sid = benchCustom.NewIDs(ctx)
		}
		benchSinkTID, benchSinkSID = tid, sid
	})
}

func BenchmarkOtelNewIDsParallel(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var tid oteltrace.TraceID
		var sid oteltrace.SpanID
		for pb.Next() {
			tid, sid = benchOtel.NewIDs(ctx)
		}
		benchSinkTID, benchSinkSID = tid, sid
	})
}

