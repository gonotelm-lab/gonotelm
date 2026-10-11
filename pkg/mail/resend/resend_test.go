package resend

import (
	"encoding/json"
	stderr "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

type capturedRequest struct {
	auth  string
	idKey string
	body  map[string]any
}

func newTestServer(t *testing.T, status int, respBody string, headers map[string]string) (*httptest.Server, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.auth = r.Header.Get("Authorization")
		captured.idKey = r.Header.Get("Idempotency-Key")

		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &captured.body)

		w.Header().Set("Content-Type", "application/json")
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)

	return srv, captured
}

func newSender(t *testing.T, baseURL string) *Sender {
	t.Helper()

	sender, err := New(
		mail.Config{
			From:     "no-reply@mail.gonotelm.com",
			FromName: "GoNoteLM",
		},
		"re_test",
		WithBaseURL(baseURL),
	)
	if err != nil {
		t.Fatal(err)
	}

	return sender
}

func TestSendPostsRenderedMessage(t *testing.T) {
	srv, captured := newTestServer(t, http.StatusOK, `{"id":"email-1"}`, nil)

	result, err := newSender(t, srv.URL).Send(t.Context(), mail.Message{
		To:             []string{"user@example.com"},
		Subject:        "verify",
		HTML:           "<p>hi</p>",
		Text:           "hi",
		Headers:        map[string]string{"Auto-Submitted": "auto-generated"},
		IdempotencyKey: "key-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Id != "email-1" {
		t.Fatalf("unexpected result id: %q", result.Id)
	}
	if captured.auth != "Bearer re_test" {
		t.Fatalf("unexpected auth header: %q", captured.auth)
	}
	if captured.idKey != "key-1" {
		t.Fatalf("caller idempotency key must be forwarded, got %q", captured.idKey)
	}
	if captured.body["from"] != "GoNoteLM <no-reply@mail.gonotelm.com>" {
		t.Fatalf("unexpected from: %v", captured.body["from"])
	}
	if captured.body["subject"] != "verify" || captured.body["html"] != "<p>hi</p>" || captured.body["text"] != "hi" {
		t.Fatalf("unexpected body: %v", captured.body)
	}

	to, _ := captured.body["to"].([]any)
	if len(to) != 1 || to[0] != "user@example.com" {
		t.Fatalf("unexpected recipients: %v", captured.body["to"])
	}

	headers, _ := captured.body["headers"].(map[string]any)
	if headers["Auto-Submitted"] != "auto-generated" {
		t.Fatalf("unexpected headers: %v", captured.body["headers"])
	}
}

func TestSendGeneratesIdempotencyKey(t *testing.T) {
	srv, captured := newTestServer(t, http.StatusOK, `{"id":"email-2"}`, nil)

	_, err := newSender(t, srv.URL).Send(t.Context(), mail.Message{
		To:      []string{"user@example.com"},
		Subject: "verify",
		Text:    "hi",
	})
	if err != nil {
		t.Fatal(err)
	}

	if captured.idKey == "" {
		t.Fatal("a random idempotency key must be sent when the caller does not provide one")
	}
}

func TestSendRateLimitedIsTransient(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusTooManyRequests, `{"message":"slow down"}`,
		map[string]string{"Retry-After": "7"})

	_, err := newSender(t, srv.URL).Send(t.Context(), mail.Message{
		To:      []string{"user@example.com"},
		Subject: "verify",
		Text:    "hi",
	})

	var rateLimit *mail.RateLimitError
	if !stderr.As(err, &rateLimit) {
		t.Fatalf("want *mail.RateLimitError, got %v", err)
	}
	if rateLimit.RetryAfter != 7*time.Second {
		t.Fatalf("unexpected retry after: %s", rateLimit.RetryAfter)
	}
	if !mail.IsTransient(err) {
		t.Fatal("rate limited must be transient")
	}
}

func TestSendRejectedRequestIsPermanent(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusUnprocessableEntity, `{"message":"domain not verified"}`, nil)

	_, err := newSender(t, srv.URL).Send(t.Context(), mail.Message{
		To:      []string{"user@example.com"},
		Subject: "verify",
		Text:    "hi",
	})
	if !mail.IsPermanent(err) {
		t.Fatalf("want a permanent failure, got %v", err)
	}
}

func TestSendRejectsIncompleteMessage(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusOK, `{"id":"email-3"}`, nil)
	sender := newSender(t, srv.URL)

	if _, err := sender.Send(t.Context(), mail.Message{Subject: "verify", Text: "hi"}); !mail.IsPermanent(err) {
		t.Fatalf("empty recipient must be a permanent failure, got %v", err)
	}
	if _, err := sender.Send(t.Context(), mail.Message{To: []string{"a@b.com"}, Text: "hi"}); !mail.IsPermanent(err) {
		t.Fatalf("empty subject must be a permanent failure, got %v", err)
	}
}

func TestNewValidatesConfig(t *testing.T) {
	if _, err := New(mail.Config{}, "re_test"); err == nil {
		t.Fatal("missing from address must fail")
	}
	if _, err := New(mail.Config{From: "no-reply@mail.gonotelm.com"}, " "); err == nil {
		t.Fatal("missing api key must fail")
	}
	if _, err := New(mail.Config{From: "email.gonotelm.com"}, "re_test"); err == nil {
		t.Fatal("a bare domain is not an address and must fail")
	}
	if _, err := New(mail.Config{From: "GoNoteLM <onboarding@resend.dev>"}, "re_test"); err == nil {
		t.Fatal("a display name in from must fail")
	}
}

func TestFormatAddress(t *testing.T) {
	cases := []struct {
		name string
		addr string
		want string
	}{
		{"GoNoteLM", "no-reply@mail.gonotelm.com", "GoNoteLM <no-reply@mail.gonotelm.com>"},
		{"", "no-reply@mail.gonotelm.com", "no-reply@mail.gonotelm.com"},
		{"Go,NoteLM", "no-reply@mail.gonotelm.com", `"Go,NoteLM" <no-reply@mail.gonotelm.com>`},
		{`Go"Note`, "no-reply@mail.gonotelm.com", `"Go\"Note" <no-reply@mail.gonotelm.com>`},
		{" GoNoteLM", "no-reply@mail.gonotelm.com", `" GoNoteLM" <no-reply@mail.gonotelm.com>`},
	}

	for _, c := range cases {
		if got := formatAddress(c.name, c.addr); got != c.want {
			t.Fatalf("formatAddress(%q, %q) = %q, want %q", c.name, c.addr, got, c.want)
		}
	}
}
