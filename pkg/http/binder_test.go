package http

import (
	stderrors "errors"
	stdhttp "net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type bindPathUUIDRequest struct {
	ID uuid.UUID `path:"id,required"`
}

func TestCanonicalBinder_BindPathUUID(t *testing.T) {
	binder := NewCanonicalBinder()
	req := &protocol.Request{}

	u := uuid.NewV7()
	params := param.Params{
		{
			Key:   "id",
			Value: u.String(),
		},
	}

	var out bindPathUUIDRequest
	if err := binder.BindPath(req, &out, params); err != nil {
		t.Fatalf("BindPath() failed: %v", err)
	}

	if out.ID.NotEqualsTo(u) {
		t.Fatalf("BindPath() id mismatch, got=%s want=%s", out.ID.String(), u.String())
	}
}

func TestCanonicalBinder_BindPathUUIDInvalid(t *testing.T) {
	binder := NewCanonicalBinder()
	req := &protocol.Request{}

	params := param.Params{
		{
			Key:   "id",
			Value: "not-a-uuid",
		},
	}

	var out bindPathUUIDRequest
	if err := binder.BindPath(req, &out, params); err == nil {
		t.Fatal("BindPath() expected error for invalid uuid, got nil")
	}
}

type bindQueryUUIDArrayRequest struct {
	IDs uuid.UUIDArray `query:"ids,required"`
}

func TestCanonicalBinder_BindQueryUUIDArray(t *testing.T) {
	binder := NewCanonicalBinder()
	req := &protocol.Request{}

	u1 := uuid.NewV7()
	u2 := uuid.NewV7()
	req.SetRequestURI("/?" + "ids=" + u1.String() + "," + u2.String())

	var out bindQueryUUIDArrayRequest
	if err := binder.BindQuery(req, &out); err != nil {
		t.Fatalf("BindQuery() failed: %v", err)
	}

	if len(out.IDs) != 2 {
		t.Fatalf("BindQuery() ids len = %d, want 2", len(out.IDs))
	}
	if out.IDs[0].NotEqualsTo(u1) || out.IDs[1].NotEqualsTo(u2) {
		t.Fatalf("BindQuery() ids mismatch, got=%v want=[%s %s]", out.IDs, u1.String(), u2.String())
	}
}

func TestCanonicalBinder_BindQueryUUIDArrayInvalid(t *testing.T) {
	binder := NewCanonicalBinder()
	req := &protocol.Request{}
	req.SetRequestURI("/?ids=" + uuid.NewV7().String() + ",not-a-uuid")

	var out bindQueryUUIDArrayRequest
	if err := binder.BindQuery(req, &out); err == nil {
		t.Fatal("BindQuery() expected error for invalid uuid array, got nil")
	}
}

type bindSelfValidatorRequest struct {
	State string `query:"state,required"`
}

func (r *bindSelfValidatorRequest) Validate(req *protocol.Request) error {
	return xerror.ErrUnauthorized.Msg("cookie state mismatch")
}

// SelfValidator errors that are already canonical must keep their http status.
func TestCanonicalBinder_ValidatePreservesInnerError(t *testing.T) {
	binder := NewCanonicalBinder()
	req := &protocol.Request{}
	req.SetRequestURI("/?state=abc")

	var out bindSelfValidatorRequest
	err := binder.Validate(req, &out)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}

	var ie *xerror.InnerError
	if !stderrors.As(err, &ie) {
		t.Fatalf("Validate() error type = %T, want *xerror.InnerError", err)
	}
	if ie.Status != stdhttp.StatusUnauthorized {
		t.Fatalf("Validate() status = %d, want %d", ie.Status, stdhttp.StatusUnauthorized)
	}
	if ie.Code != xerror.CodeUnauthorized {
		t.Fatalf("Validate() code = %d, want %d", ie.Code, xerror.CodeUnauthorized)
	}
}

// Plain (non canonical) errors still become a 200/INVALID_PARAMETERS result.
func TestCanonicalBinder_BindErrorBecomesInvalidParams(t *testing.T) {
	binder := NewCanonicalBinder()
	req := &protocol.Request{}
	req.SetRequestURI("/?ids=not-a-uuid")

	var out bindQueryUUIDArrayRequest
	err := binder.BindQuery(req, &out)
	if err == nil {
		t.Fatal("BindQuery() expected error, got nil")
	}

	var ie *xerror.InnerError
	if !stderrors.As(err, &ie) {
		t.Fatalf("BindQuery() error type = %T, want *xerror.InnerError", err)
	}
	if ie.Status != stdhttp.StatusOK || ie.Code != xerror.CodeInvalidParams {
		t.Fatalf("BindQuery() got status=%d code=%d, want status=%d code=%d",
			ie.Status, ie.Code, stdhttp.StatusOK, xerror.CodeInvalidParams)
	}
}
