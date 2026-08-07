package trace

import (
	"context"
	"testing"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"

	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestIdGen(t *testing.T) {
	idg := IDGenerator{}
	tid, sid := idg.NewIDs(context.Background())
	if !tid.IsValid() || !sid.IsValid() {
		t.Fatalf("invalid ids: %s %s", tid, sid)
	}
}

func TestIdGenWithReqId(t *testing.T) {
	reqId := requestid.Gen()
	ctx := pkgcontext.WithReqId(t.Context(), reqId)

	idg := IDGenerator{}
	tid, sid := idg.NewIDs(ctx)
	if tid != oteltrace.TraceID(reqId) {
		t.Fatalf("trace id mismatch: got %s want %s", tid, reqId)
	}
	if !sid.IsValid() {
		t.Fatalf("invalid span id: %s", sid)
	}
}

func TestIdGenWithZeroReqId(t *testing.T) {
	ctx := pkgcontext.WithReqId(t.Context(), requestid.ID{})

	idg := IDGenerator{}
	tid, sid := idg.NewIDs(ctx)
	if !tid.IsValid() || !sid.IsValid() {
		t.Fatalf("invalid ids: %s %s", tid, sid)
	}
}
