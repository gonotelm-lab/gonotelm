package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// ResendHandler 重发验证信：只重置两条验证凭证，密码与票据保持不变。
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
