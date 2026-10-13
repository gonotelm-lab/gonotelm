package entity

import (
	"testing"
)

func TestPasswordResetPendingCredentials(t *testing.T) {
	pending := NewPasswordResetPending("User@Example.COM")

	if pending.Email != "user@example.com" {
		t.Fatalf("email = %q, want normalized lower-case", pending.Email)
	}
	// 找回密码请求本身不带密码，记录里也不能有
	if pending.LinkTokenHash != "" {
		t.Fatal("a fresh pending must not carry credentials yet")
	}

	linkToken, err := pending.IssueCredentials()
	if err != nil {
		t.Fatalf("IssueCredentials failed: %v", err)
	}
	if linkToken == "" {
		t.Fatal("token must be returned to the caller for the email")
	}
	if pending.LinkTokenHash != HashSecret(linkToken) {
		t.Fatal("link token must be stored hashed")
	}
	if pending.LinkTokenHash == linkToken {
		t.Fatal("record must not keep the plaintext token")
	}
}

// 重新发起要换掉链接 token，旧链接不能继续可用
func TestPasswordResetPendingRotatesCredentials(t *testing.T) {
	pending := NewPasswordResetPending("user@example.com")

	first, err := pending.IssueCredentials()
	if err != nil {
		t.Fatalf("first IssueCredentials failed: %v", err)
	}
	firstHash := pending.LinkTokenHash

	second, err := pending.IssueCredentials()
	if err != nil {
		t.Fatalf("second IssueCredentials failed: %v", err)
	}

	if second == first {
		t.Fatal("re-issuing must produce a new token")
	}
	if pending.LinkTokenHash != HashSecret(second) || pending.LinkTokenHash == firstHash {
		t.Fatal("link token hash must be replaced")
	}
}

// 重置链接比注册验证短命：点开一次就能改密码
func TestPasswordResetPendingTTLShorterThanSignup(t *testing.T) {
	if PasswordResetPendingTTL >= SignupPendingTTL {
		t.Fatalf("password reset TTL (%s) must be shorter than signup TTL (%s)",
			PasswordResetPendingTTL, SignupPendingTTL)
	}
}
