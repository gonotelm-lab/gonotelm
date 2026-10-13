package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// ResendHandler 重发验证邮件：只重置链接 token 和验证码，密码与 ticket 不变
type ResendHandler struct {
	signupService *identityservice.SignupService
	mailer        adapter.Mailer
}

func NewResendHandler(
	signupService *identityservice.SignupService,
	mailer adapter.Mailer,
) *ResendHandler {
	return &ResendHandler{
		signupService: signupService,
		mailer:        mailer,
	}
}

type ResendCommand struct {
	Email string
}

type ResendResult struct{}

func (h *ResendHandler) Handle(ctx context.Context, cmd *ResendCommand) (*ResendResult, error) {
	dispatch, err := h.signupService.Resend(ctx, cmd.Email)
	if err != nil {
		return nil, err
	}

	if err := sendVerification(ctx, h.mailer, dispatch); err != nil {
		return nil, err
	}

	return &ResendResult{}, nil
}
