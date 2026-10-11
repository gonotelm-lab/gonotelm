package adapter

import (
	"testing"
)

func TestLogMailer_SendEmailVerification(t *testing.T) {
	if err := NewLogMailer().SendEmailVerification(t.Context(), "dev@example.com", "https://example.com/verify?token=1", "123456"); err != nil {
		t.Fatal(err)
	}
}
