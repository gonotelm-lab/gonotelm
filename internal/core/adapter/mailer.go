package adapter

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

// Mailer 通用邮件发送
type Mailer interface {
	Send(ctx context.Context, msg mail.Message) error
}
