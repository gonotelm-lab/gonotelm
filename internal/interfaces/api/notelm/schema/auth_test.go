package schema

import (
	"encoding/json"
	"fmt"
	"strings"
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

// 认证请求带密码与一次性凭证，用 %+v 打印整个请求体时只能出现占位符。
// 值和指针两种形态都断言：handler 拿的是指针，但把值打出来（%+v 打 *req）也不能漏。
func TestAuthRequestStringRedactsCredentials(t *testing.T) {
	register := EmailRegisterRequest{Email: "user@example.com", Password: "s3cret-pass"}
	verifyCode := EmailVerifyCodeRequest{Email: "user@example.com", Code: "123456"}
	login := PasswordLoginRequest{Email: "user@example.com", Password: "s3cret-pass"}
	setPassword := SetPasswordRequest{OldPassword: "old-pass", NewPassword: "new-pass"}
	resetLink := PasswordResetConfirmRequest{Token: "reset-token-abc", NewPassword: "new-pass"}
	callback := AuthCallbackRequest{State: "state-abc", Code: "code-abc", Error: "access_denied"}

	cases := []struct {
		name   string
		ptr    fmt.Stringer
		val    any
		secret string
		// keep 非空时必须出现在输出里，防止 String 被写成常量而丢掉可读信息
		keep string
	}{
		{"register password", &register, register, "s3cret-pass", "user@example.com"},
		{"verify code", &verifyCode, verifyCode, "123456", "user@example.com"},
		{"password login", &login, login, "s3cret-pass", "user@example.com"},
		{"set password old", &setPassword, setPassword, "old-pass", ""},
		{"set password new", &setPassword, setPassword, "new-pass", ""},
		{"reset confirm token", &resetLink, resetLink, "reset-token-abc", ""},
		{"reset confirm password", &resetLink, resetLink, "new-pass", ""},
		{"callback state", &callback, callback, "state-abc", ""},
		{"callback code", &callback, callback, "code-abc", "access_denied"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, printed := range []any{tc.ptr, tc.val} {
				got := fmt.Sprintf("%+v", printed)
				if strings.Contains(got, tc.secret) {
					t.Fatalf("credential leaked into print output: %s", got)
				}
				if !strings.Contains(got, redactedValue) {
					t.Fatalf("missing redaction placeholder: %s", got)
				}
				if tc.keep != "" && !strings.Contains(got, tc.keep) {
					t.Fatalf("non-credential field dropped from print output: %s", got)
				}
			}
		})
	}
}

// 重置页与注册页共用同一份密码策略契约，字段名与含义都不能各自漂
func TestPasswordResetPrefetchResponseSharesRegisterContract(t *testing.T) {
	resetBody, err := json.Marshal(PasswordResetPrefetchResponse{RequireComplexPassword: true})
	if err != nil {
		t.Fatalf("marshal reset prefetch failed: %v", err)
	}
	registerBody, err := json.Marshal(EmailRegisterPrefetchResponse{RequireComplexPassword: true})
	if err != nil {
		t.Fatalf("marshal register prefetch failed: %v", err)
	}

	if string(resetBody) != string(registerBody) {
		t.Fatalf("prefetch bodies diverged: reset=%s register=%s", resetBody, registerBody)
	}
	if string(resetBody) != `{"require_complex_password":true}` {
		t.Fatalf("unexpected prefetch body: %s", resetBody)
	}
}

// 找回密码的 DTO 校验：邮箱先归一化再判格式，凭证与密码都必填
func TestPasswordResetRequestsValidate(t *testing.T) {
	if err := (&PasswordResetRequest{Email: "  E2E.User@Example.com  "}).Validate(); err != nil {
		t.Fatalf("a valid email should pass after normalization, got %v", err)
	}
	if err := (&PasswordResetRequest{Email: "nope"}).Validate(); !pkgerrors.Is(err, identerr.ErrEmailInvalid) {
		t.Fatalf("bad email should be reported as ErrEmailInvalid, got %v", err)
	}

	if err := (&PasswordResetConfirmRequest{NewPassword: "s3cret-pass"}).Validate(); err == nil {
		t.Fatal("link confirm must require the token")
	}
	if err := (&PasswordResetConfirmRequest{Token: "reset-token"}).Validate(); err == nil {
		t.Fatal("link confirm must require the new password")
	}
	if err := (&PasswordResetConfirmRequest{
		Token: "reset-token", NewPassword: "s3cret-pass",
	}).Validate(); err != nil {
		t.Fatalf("valid link confirm should pass, got %v", err)
	}
}
