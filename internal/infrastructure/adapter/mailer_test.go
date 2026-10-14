package adapter

import (
	"context"
	"errors"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

type recordingSender struct {
	sent []mail.Message
	err  error
}

func (r *recordingSender) Send(_ context.Context, msg mail.Message) (*mail.Result, error) {
	r.sent = append(r.sent, msg)
	if r.err != nil {
		return nil, r.err
	}

	return &mail.Result{Id: "rec-1"}, nil
}

func TestMailerSendDelegatesToSender(t *testing.T) {
	rec := &recordingSender{}
	msg := mail.Message{To: []string{"user@example.com"}, Subject: "hello"}

	if err := NewMailer(rec).Send(t.Context(), msg); err != nil {
		t.Fatal(err)
	}

	if len(rec.sent) != 1 || rec.sent[0].Subject != "hello" {
		t.Fatalf("unexpected sent mails: %+v", rec.sent)
	}
}

func TestMailerSendPropagatesSenderError(t *testing.T) {
	sentinel := errors.New("boom")

	err := NewMailer(&recordingSender{err: sentinel}).Send(t.Context(), mail.Message{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want the sender error, got %v", err)
	}
}
