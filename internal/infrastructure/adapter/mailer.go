package adapter

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
)

// LogMailer 开发用实现：只把邮件内容打到日志，不真正发送。
type LogMailer struct{}

var _ adapter.Mailer = (*LogMailer)(nil)

func NewLogMailer() *LogMailer {
	return &LogMailer{}
}

func (m *LogMailer) SendEmailVerification(ctx context.Context, to, link, code string) error {
	slog.InfoContext(ctx, "dev mailer: send email verification",
		slog.String("to", to),
		slog.String("link", link),
		slog.String("code", code),
	)
	return nil
}
