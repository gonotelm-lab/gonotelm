package safe

import (
	"context"
	"errors"
	"testing"

	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type doCtxKey string

func TestDoWithContext_ReturnsUnderlyingError(t *testing.T) {
	want := errors.New("boom")

	if got := DoWithContext(context.Background(), func(context.Context) error { return want }); !errors.Is(got, want) {
		t.Fatalf("DoWithContext returned %v, want %v", got, want)
	}
}

func TestDoWithContext_PassesContext(t *testing.T) {
	const key doCtxKey = "k"
	ctx := context.WithValue(context.Background(), key, "call-value")

	err := DoWithContext(ctx, func(ctx context.Context) error {
		if got := ctx.Value(key); got != "call-value" {
			t.Fatalf("wrapped fn received ctx.Value = %v, want %q", got, "call-value")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("DoWithContext returned %v, want nil", err)
	}
}

// Mirrors Do: a recovered panic must surface as a non-nil error so callers can
// tell that the function did not complete successfully.
func TestDoWithContext_RecoversPanicAsError(t *testing.T) {
	err := DoWithContext(context.Background(), func(context.Context) error {
		panic("boom")
	})

	if err == nil {
		t.Fatal("after recovering a panic, DoWithContext returned nil, want a non-nil error")
	}
}

func TestDo_RecoversPanicAsErrInner(t *testing.T) {
	err := Do(context.Background(), func() error { panic("boom") })
	if !errors.Is(err, pkgerrors.ErrInner) {
		t.Fatalf("Do returned %v, want ErrInner", err)
	}
}
