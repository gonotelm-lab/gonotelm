// Package log implements mail.Sender by writing the email to the log instead of
// sending it. It is the dev sender and the fallback when a real sender cannot be
// built, which is why printing the body (link and code) is acceptable.
package log

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

type Sender struct {
	level slog.Level
}

var _ mail.Sender = (*Sender)(nil)

// New builds the log sender; every email is written at level.
func New(level slog.Level) *Sender {
	return &Sender{level: level}
}

func (s *Sender) Send(ctx context.Context, msg mail.Message) (*mail.Result, error) {
	slog.Log(ctx, s.level, "log mailer: mail not actually sent",
		slog.String("to", strings.Join(msg.To, ",")),
		slog.String("subject", msg.Subject),
		slog.String("text", msg.Text),
	)

	return &mail.Result{Id: "log"}, nil
}
