package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// VerifyHandler 处理邮箱验证：邮件里只发一个 6 位验证码，用户回到验证页输入
// 验证码只有 6 位，所以要限制 attempts 次数与 15 分钟 TTL
type VerifyHandler struct {
	*baseHandler
	signupService *identityservice.SignupService
}

func NewVerifyHandler(
	signupService *identityservice.SignupService,
	userService *identityservice.UserService,
	userSessionRepo identrepo.UserSessionRepository,
) *VerifyHandler {
	return &VerifyHandler{
		baseHandler:   newBaseHandler(userService, userSessionRepo),
		signupService: signupService,
	}
}

type VerifyCommand struct {
	Email  string
	Code   string
	Device entity.DeviceType
	// Ticket 发起注册那个浏览器的 ticket，不匹配就不写密码
	Ticket string
}

type VerifyResult = SessionResult

// VerifyCode 验证码验证：email + 6 位验证码取出 pending 后建号
func (h *VerifyHandler) VerifyCode(ctx context.Context, cmd *VerifyCommand) (*VerifyResult, error) {
	agg, err := h.signupService.VerifyCode(ctx, cmd.Email, cmd.Code, cmd.Ticket)
	if err != nil {
		return nil, err
	}

	return h.issueVerifiedSession(ctx, agg, cmd.Device)
}

// issueVerifiedSession 建号成功后创建邮箱来源的会话
func (h *VerifyHandler) issueVerifiedSession(
	ctx context.Context,
	agg *entity.UserAggregate,
	device entity.DeviceType,
) (*VerifyResult, error) {
	session, err := h.issueSession(ctx, agg.User(), device, entity.LoginSourceEmail)
	if err != nil {
		return nil, err
	}

	return &VerifyResult{SessionId: session.Id, UserId: agg.Id().String()}, nil
}
