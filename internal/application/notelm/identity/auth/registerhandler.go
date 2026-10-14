package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// RegisterHandler 邮箱注册，是否走邮件验证由 auth.emailVerificationRequired 决定
// 需要验证时返回凭证并发送验证邮件，验证通过才创建账号
// 免验证时邮箱视为已验证，直接创建账号并创建会话
type RegisterHandler struct {
	*baseHandler
	signupService *identityservice.SignupService
	mailer        adapter.Mailer
}

func NewRegisterHandler(
	signupService *identityservice.SignupService,
	userService *identityservice.UserService,
	userSessionRepo identrepo.UserSessionRepository,
	mailer adapter.Mailer,
) *RegisterHandler {
	return &RegisterHandler{
		baseHandler:   newBaseHandler(userService, userSessionRepo),
		signupService: signupService,
		mailer:        mailer,
	}
}

type RegisterCommand struct {
	Email    string
	Password string
	Device   identityentity.DeviceType
}

type RegisterResult struct {
	// Ticket 非空时由 interfaces 层写进 _gnlm_signup_ticket
	// 重复注册沿用旧 ticket，不重置 cookie
	Ticket string
	// Session 非空时 interfaces 层写会话 cookie
	Session *SessionResult
}

func (h *RegisterHandler) Handle(ctx context.Context, cmd *RegisterCommand) (*RegisterResult, error) {
	if err := validatePasswordPolicy(cmd.Password); err != nil {
		return nil, err
	}

	if !conf.NotelmGlobal().Auth.EmailVerificationRequired {
		return h.registerVerified(ctx, cmd)
	}

	dispatch, err := h.signupService.Start(ctx, cmd.Email, cmd.Password)
	if err != nil {
		return nil, err
	}

	if err := sendVerification(ctx, h.mailer, dispatch); err != nil {
		return nil, err
	}

	return &RegisterResult{Ticket: dispatch.Ticket}, nil
}

// registerVerified 免验证注册：创建账号后创建会话，创建失败不回滚账号
func (h *RegisterHandler) registerVerified(
	ctx context.Context,
	cmd *RegisterCommand,
) (*RegisterResult, error) {
	agg, err := h.signupService.DirectSignup(ctx, cmd.Email, cmd.Password)
	if err != nil {
		return nil, err
	}

	session, err := h.issueSession(ctx, agg.User(), cmd.Device, identityentity.LoginSourceEmail)
	if err != nil {
		return nil, err
	}

	return &RegisterResult{Session: &SessionResult{
		SessionId: session.Id,
		UserId:    agg.Id().String(),
	}}, nil
}

// sendVerification 发送验证码邮件，失败撤销本次占用的发送间隔，统一返回 ErrMailSendFailed
func sendVerification(
	ctx context.Context,
	mailer adapter.Mailer,
	dispatch *identityservice.SignupDispatch,
) error {
	if err := mailer.SendEmailVerification(ctx, dispatch.Email, dispatch.Code); err != nil {
		dispatch.Rollback(ctx)
		return errors.Wrapf(identerr.ErrMailSendFailed, "send verification mail: %s", err.Error())
	}

	return nil
}
