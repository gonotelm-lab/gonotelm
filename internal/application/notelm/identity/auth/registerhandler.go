package auth

import (
	"context"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// RegisterHandler 邮箱注册第一步：委托 SignupService 下发并落库验证凭证，再发出验证信。
type RegisterHandler struct {
	signupService *identityservice.SignupService
	mailer        adapter.Mailer
	verifyURLBase string
}

func NewRegisterHandler(
	signupService *identityservice.SignupService,
	mailer adapter.Mailer,
	verifyURLBase string,
) *RegisterHandler {
	return &RegisterHandler{
		signupService: signupService,
		mailer:        mailer,
		verifyURLBase: verifyURLBase,
	}
}

type RegisterCommand struct {
	Email    string
	Password string
}

type RegisterResult struct {
	// Ticket 非空时由 interfaces 层写进 _gnlm_signup_ticket；
	// 重复注册会保留首次票据，不重置 cookie。
	Ticket string
}

func (h *RegisterHandler) Handle(ctx context.Context, cmd *RegisterCommand) (*RegisterResult, error) {
	dispatch, err := h.signupService.Start(ctx, cmd.Email, cmd.Password)
	if err != nil {
		return nil, err
	}

	if err := sendVerification(ctx, h.mailer, h.verifyURLBase, dispatch); err != nil {
		return nil, err
	}

	return &RegisterResult{Ticket: dispatch.Ticket}, nil
}

// sendVerification 拼验证链接并发送验证信；失败撤销本次占用的发信冷却位后归一化为 ErrMailSendFailed。
func sendVerification(
	ctx context.Context,
	mailer adapter.Mailer,
	verifyURLBase string,
	dispatch *identityservice.SignupDispatch,
) error {
	link := buildVerifyLink(verifyURLBase, dispatch.LinkToken)
	if err := mailer.SendEmailVerification(ctx, dispatch.Email, link, dispatch.Code); err != nil {
		dispatch.Rollback(ctx)
		return errors.Wrapf(identerr.ErrMailSendFailed, "send verification mail: %s", err.Error())
	}

	return nil
}

// buildVerifyLink 验证链接落到前端页面，token 放 URL 片段，不进服务端日志也不进 Referer。
func buildVerifyLink(base, linkToken string) string {
	return strings.TrimRight(base, "/") + "#token=" + linkToken
}
