package auth

import (
	"context"

	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// ResendMailHandler 重发验证邮件：只重置验证码，密码与 ticket 不变
type ResendMailHandler struct {
	signupService *identityservice.SignupService
	mailer        identrepo.EmailVerificationMailer
}

func NewResendMailHandler(
	signupService *identityservice.SignupService,
	mailer identrepo.EmailVerificationMailer,
) *ResendMailHandler {
	return &ResendMailHandler{
		signupService: signupService,
		mailer:        mailer,
	}
}

type ResendCommand struct {
	Email string
}

type ResendResult struct{}

func (h *ResendMailHandler) Handle(ctx context.Context, cmd *ResendCommand) (*ResendResult, error) {
	dispatch, err := h.signupService.Resend(ctx, cmd.Email)
	if err != nil {
		return nil, err
	}

	if err := sendVerification(ctx, h.mailer, dispatch); err != nil {
		return nil, err
	}

	return &ResendResult{}, nil
}
