package entity

import (
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func TestValidateEmail(t *testing.T) {
	valid := []string{
		"user@example.com",
		"e2e.user+tag@sub.example.co",
		"a_b-c@my-domain.io",
	}
	invalid := []string{
		"",
		"user",
		"user@",
		"@example.com",
		"user@localhost",
		"user@@example.com",
		"user name@example.com",
		"user@example",
		"user@example.c",
		"user@exa mple.com",
		"user@example.123",
		"user@-example.com",
		"user@example..com",
		strings.Repeat("a", 250) + "@example.com",
	}

	for _, email := range valid {
		if err := ValidateEmail(email); err != nil {
			t.Fatalf("ValidateEmail(%q) = %v, want nil", email, err)
		}
	}
	for _, email := range invalid {
		if err := ValidateEmail(email); !pkgerrors.Is(err, identerr.ErrEmailInvalid) {
			t.Fatalf("ValidateEmail(%q) = %v, want ErrEmailInvalid", email, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("1234567"); !pkgerrors.Is(err, identerr.ErrWeakPassword) {
		t.Fatalf("7 bytes should be rejected, got %v", err)
	}
	if err := ValidatePassword("12345678"); err != nil {
		t.Fatalf("8 bytes should pass, got %v", err)
	}
	if err := ValidatePassword(strings.Repeat("a", MaxPasswordBytes)); err != nil {
		t.Fatalf("%d bytes should pass, got %v", MaxPasswordBytes, err)
	}
	if err := ValidatePassword(strings.Repeat("a", MaxPasswordBytes+1)); !pkgerrors.Is(err, identerr.ErrWeakPassword) {
		t.Fatalf("over bcrypt limit should be rejected, got %v", err)
	}
}

func TestValidateComplexPassword(t *testing.T) {
	for _, weak := range []string{"12345678", "abcdefgh", "Abcdefgh", "abcd1234", "Ab1"} {
		if err := ValidateComplexPassword(weak); !pkgerrors.Is(err, identerr.ErrWeakPassword) {
			t.Fatalf("ValidateComplexPassword(%q) = %v, want ErrWeakPassword", weak, err)
		}
	}

	for _, strong := range []string{"Abcd1234", "aaaaAAAA1", "P@ssw0rd"} {
		if err := ValidateComplexPassword(strong); err != nil {
			t.Fatalf("ValidateComplexPassword(%q) = %v, want nil", strong, err)
		}
	}
}

func TestSignupPendingCredentials(t *testing.T) {
	pending := NewSignupPending("User@Example.COM", "bcrypt-hash", "raw-ticket")

	if pending.Email != "user@example.com" {
		t.Fatalf("email = %q, want normalized lower-case", pending.Email)
	}
	if pending.TicketHash != HashSecret("raw-ticket") {
		t.Fatal("ticket must be stored hashed")
	}
	if !pending.MatchesTicket("raw-ticket") || pending.MatchesTicket("other-ticket") {
		t.Fatal("ticket match is wrong")
	}

	code, err := pending.IssueCredentials()
	if err != nil {
		t.Fatalf("IssueCredentials failed: %v", err)
	}
	if pending.CodeHash != HashSecret(code) {
		t.Fatal("code must be stored hashed")
	}
	if pending.CodeHash == code {
		t.Fatal("record must not keep the plaintext code")
	}
	if len(code) != 6 {
		t.Fatalf("code = %q, want 6 digits", code)
	}
	if !pending.MatchesCode(code) {
		t.Fatal("issue then match should succeed")
	}
	if pending.MatchesCode("000000") && code != "000000" {
		t.Fatal("wrong code must not match")
	}

	// 验证码过期后即使值正确也不再匹配
	pending.CodeExpireAt = valobj.NewTimeFrom(time.Now().Add(-time.Minute).UnixMilli())
	if pending.MatchesCode(code) {
		t.Fatal("expired code must not match")
	}
}
