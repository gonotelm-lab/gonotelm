package schema

import (
	"testing"

	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func TestEmailRegisterRequestRejectsBadEmail(t *testing.T) {
	bad := []string{"", "user", "user@localhost", "user@example", "user @example.com", "user@example.c"}
	for _, email := range bad {
		req := &EmailRegisterRequest{Email: email, Password: "s3cret-pass"}
		if err := req.Validate(); !pkgerrors.Is(err, identerr.ErrEmailInvalid) {
			t.Fatalf("Validate(%q) = %v, want ErrEmailInvalid", email, err)
		}
	}

	req := &EmailRegisterRequest{Email: "user@example.com", Password: ""}
	if err := req.Validate(); pkgerrors.Is(err, identerr.ErrEmailInvalid) {
		t.Fatal("missing password must not be reported as an email error")
	}

	req = &EmailRegisterRequest{Email: "E2E.User@Example.com", Password: "s3cret-pass"}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestEmailRequestsValidateEmail(t *testing.T) {
	if err := (&EmailResendRequest{Email: "nope"}).Validate(); !pkgerrors.Is(err, identerr.ErrEmailInvalid) {
		t.Fatalf("resend should reject a bad email, got %v", err)
	}
	if err := (&EmailVerifyCodeRequest{Email: "nope", Code: "123456"}).Validate(); !pkgerrors.Is(err, identerr.ErrEmailInvalid) {
		t.Fatalf("verify code should reject a bad email, got %v", err)
	}
	if err := (&EmailVerifyCodeRequest{Email: "user@example.com", Code: ""}).Validate(); err == nil {
		t.Fatal("verify code should require the code")
	}
}

func TestIsSafeReturnTo(t *testing.T) {
	allowed := []string{"https://app.example.com", "http://localhost:5173"}

	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"empty", "", true},
		{"relative", "/dashboard", true},
		{"relative with query", "/a/b?x=1", true},
		{"protocol relative", "//evil.com", false},
		{"backslash", "/\\evil.com", false},
		{"absolute allowed", "https://app.example.com/callback", true},
		{"absolute allowed with port", "http://localhost:5173/x", true},
		{"absolute disallowed", "https://evil.com", false},
		{"scheme relative", "https:/evil.com", false},
		{"javascript", "javascript:alert(1)", false},
		{"crlf", "/a\r\nSet-Cookie:x", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSafeReturnTo(tc.raw, allowed); got != tc.want {
				t.Fatalf("IsSafeReturnTo(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
