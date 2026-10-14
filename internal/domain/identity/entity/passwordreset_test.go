package entity

import (
	"testing"
)

func TestPasswordResetStateCredentials(t *testing.T) {
	state := NewPasswordResetState("User@Example.COM")

	if state.Email != "user@example.com" {
		t.Fatalf("email = %q, want normalized lower-case", state.Email)
	}
	// 找回密码请求本身不带密码，记录里也不能有
	if state.LinkTokenHash != "" {
		t.Fatal("a fresh state must not carry credentials yet")
	}

	linkToken, err := state.IssueCredentials()
	if err != nil {
		t.Fatalf("IssueCredentials failed: %v", err)
	}
	if linkToken == "" {
		t.Fatal("token must be returned to the caller for the email")
	}
	if state.LinkTokenHash != HashSecret(linkToken) {
		t.Fatal("link token must be stored hashed")
	}
	if state.LinkTokenHash == linkToken {
		t.Fatal("record must not keep the plaintext token")
	}
}

// 重新发起要换掉链接 token，旧链接不能继续可用
func TestPasswordResetStateRotatesCredentials(t *testing.T) {
	state := NewPasswordResetState("user@example.com")

	first, err := state.IssueCredentials()
	if err != nil {
		t.Fatalf("first IssueCredentials failed: %v", err)
	}
	firstHash := state.LinkTokenHash

	second, err := state.IssueCredentials()
	if err != nil {
		t.Fatalf("second IssueCredentials failed: %v", err)
	}

	if second == first {
		t.Fatal("re-issuing must produce a new token")
	}
	if state.LinkTokenHash != HashSecret(second) || state.LinkTokenHash == firstHash {
		t.Fatal("link token hash must be replaced")
	}
}

// 重置链接比注册验证短命：点开一次就能改密码
func TestPasswordResetStateTTLShorterThanSignup(t *testing.T) {
	if PasswordResetStateTTL >= SignupStateTTL {
		t.Fatalf("password reset TTL (%s) must be shorter than signup TTL (%s)",
			PasswordResetStateTTL, SignupStateTTL)
	}
}
