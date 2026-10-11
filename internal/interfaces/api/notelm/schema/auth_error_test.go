package schema

import (
	stderrors "errors"
	"testing"

	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func TestCallbackReasonMapsDomainErrors(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{identerr.ErrOAuthStateNotFound, "oauth-state-not-found"},
		{identerr.ErrOAuthStateMismatch, "oauth-state-mismatch"},
		{identerr.ErrIDPExchangeError, "idp-exchange-error"},
		{identerr.ErrIdpEmailUnavailable, "idp-email-unavailable"},
		{identerr.ErrUserBanned, "user-banned"},
		{identerr.ErrIdentityAlreadyLinked, "identity-already-linked"},
		{identerr.ErrProviderAlreadyBound, "provider-already-bound"},
		// 回调链路里错误都是 WithMessage 包过一层再上来的
		{pkgerrors.WithMessage(identerr.ErrUserBanned, "resolve user failed"), "user-banned"},
		{stderrors.New("boom"), CallbackReasonInternal},
	}

	for _, tc := range cases {
		if got := CallbackReason(tc.err); got != tc.want {
			t.Fatalf("CallbackReason(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestAuthCallbackErrorQueryEncode(t *testing.T) {
	q := AuthCallbackErrorQuery{Reason: "idp-exchange-error", Provider: "github"}

	if got, want := q.Encode(), "provider=github&reason=idp-exchange-error"; got != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}

	q.Settings = CallbackSettingsAccount
	if got, want := q.Encode(), "provider=github&reason=idp-exchange-error&settings=account"; got != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}
}

func TestAuthCallbackErrorQueryAppendTo(t *testing.T) {
	q := AuthCallbackErrorQuery{Reason: "idp-error", Provider: "google", Settings: CallbackSettingsAccount}

	cases := []struct {
		raw  string
		want string
	}{
		{"/settings", "/settings?provider=google&reason=idp-error&settings=account"},
		// 已有 query 与 fragment 必须保留，同名参数被覆盖
		{"/settings?tab=security&settings=profile#bind", "/settings?provider=google&reason=idp-error&settings=account&tab=security#bind"},
		{"http://127.0.0.1:5173/login", "http://127.0.0.1:5173/login?provider=google&reason=idp-error&settings=account"},
		{"http://127.0.0.1:5173/?settings=account", "http://127.0.0.1:5173/?provider=google&reason=idp-error&settings=account"},
	}

	for _, tc := range cases {
		got, err := q.AppendTo(tc.raw)
		if err != nil {
			t.Fatalf("AppendTo(%q) error: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("AppendTo(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
