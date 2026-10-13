package auth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// PasswordResetHandler 找回密码
// Handle 一律返回成功，账号不存在也照常返回，不给调用方判断邮箱是否注册的机会
// ConfirmLink 确认后重设密码并作废该账号全部会话；
// 新密码要让用户回登录页自己登一次，所以成功不回 body
type PasswordResetHandler struct {
	resetService *identityservice.PasswordResetService
	sessionRepo  identrepo.UserSessionRepository
	mailer       adapter.Mailer
}

func NewPasswordResetHandler(
	resetService *identityservice.PasswordResetService,
	userSessionRepo identrepo.UserSessionRepository,
	mailer adapter.Mailer,
) *PasswordResetHandler {
	return &PasswordResetHandler{
		resetService: resetService,
		sessionRepo:  userSessionRepo,
		mailer:       mailer,
	}
}

type PasswordResetCommand struct {
	Email string
}

type PasswordResetResult struct{}

// Handle 发起找回：只有账号存在才真发信，其余情况什么都不做也返回成功
func (h *PasswordResetHandler) Handle(
	ctx context.Context,
	cmd *PasswordResetCommand,
) (*PasswordResetResult, error) {
	dispatch, err := h.resetService.Start(ctx, cmd.Email)
	if err != nil {
		return nil, err
	}

	// 账号不存在：不写 pending 也不发信，返回结果和成功时一样
	if !dispatch.Sent {
		return &PasswordResetResult{}, nil
	}

	if err := h.sendReset(ctx, dispatch); err != nil {
		return nil, err
	}

	return &PasswordResetResult{}, nil
}

type PasswordResetConfirmCommand struct {
	Token       string
	NewPassword string
}

// ConfirmLink 链接确认：token 由前端从重置链接的 URL 片段读出后提交
// 成功只回错误码，不回 body：新密码要用户回登录页自己登一次才算数
func (h *PasswordResetHandler) ConfirmLink(
	ctx context.Context,
	cmd *PasswordResetConfirmCommand,
) error {
	if err := validatePasswordPolicy(cmd.NewPassword); err != nil {
		return err
	}

	userId, err := h.resetService.ConfirmLink(ctx, cmd.Token, cmd.NewPassword)
	if err != nil {
		return err
	}

	h.revokeSessions(ctx, userId)

	return nil
}

// revokeSessions 重设成功后作废该账号全部会话
// 重置多半发生在没有登录态的浏览器里，这一步防的是别处可能还留着的会话，
// 不是「把当前用户登出」。作废失败不回滚密码，用户拿新密码登录即可
func (h *PasswordResetHandler) revokeSessions(ctx context.Context, userId valobj.Uid) {
	if err := h.sessionRepo.DeleteByUserId(ctx, userId); err != nil {
		slog.ErrorContext(ctx, "revoke sessions after password reset failed",
			slog.String("user_id", userId.String()),
			slog.Any("err", err),
		)
	}
}

// sendReset 拼重置链接并发送邮件，失败撤销本次占用的发送间隔，统一返回 ErrMailSendFailed
func (h *PasswordResetHandler) sendReset(
	ctx context.Context,
	dispatch *identityservice.PasswordResetDispatch,
) error {
	link := buildResetLink(conf.NotelmGlobal().Auth.PasswordResetURLBase, dispatch.LinkToken)
	if err := h.mailer.SendPasswordReset(ctx, dispatch.Email, link); err != nil {
		dispatch.Rollback(ctx)

		return errors.Wrapf(identerr.ErrMailSendFailed, "send password reset mail: %s", err.Error())
	}

	return nil
}

// buildResetLink 重置链接指向前端页面，token 放 URL 片段，不进服务端日志与 Referer
func buildResetLink(base, linkToken string) string {
	return strings.TrimRight(base, "/") + "#token=" + linkToken
}
