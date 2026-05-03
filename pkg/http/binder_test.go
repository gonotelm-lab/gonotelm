package http

import (
	"net/url"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type bindPathUUIDRequest struct {
	ID uuid.UUID `path:"id,required"`
}

type bindQueryUUIDRequest struct {
	ID uuid.UUID `query:"id,required"`
}

type bindHeaderUUIDRequest struct {
	ID uuid.UUID `header:"x-id,required"`
}

type bindFormUUIDRequest struct {
	ID uuid.UUID `form:"id,required"`
}

type bindJSONUUIDRequest struct {
	ID uuid.UUID `json:"id,required"`
}

type bindAllUUIDRequest struct {
	PathID   uuid.UUID `path:"path_id,required"`
	QueryID  uuid.UUID `query:"query_id,required"`
	HeaderID uuid.UUID `header:"x-id,required"`
	FormID   uuid.UUID `form:"form_id,required"`
	JSONID   uuid.UUID `json:"json_id,required"`
}

type bindPathURLRequest struct {
	URL url.URL `path:"u,required"`
}

type bindQueryURLRequest struct {
	URL url.URL `query:"u,required"`
}

type bindHeaderURLRequest struct {
	URL url.URL `header:"x-url,required"`
}

type bindFormURLRequest struct {
	URL url.URL `form:"u,required"`
}

type bindAllURLRequest struct {
	PathURL   url.URL `path:"path_url,required"`
	QueryURL  url.URL `query:"query_url,required"`
	HeaderURL url.URL `header:"x-url,required"`
	FormURL   url.URL `form:"form_url,required"`
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
