package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// VerifyHandler 统一处理邮箱验证的两条入口，验证通过后创建会话
// VerifyLink 邮件链接里的 token 本身即凭据，用它取出 pending
// VerifyCode email + 6 位验证码取出 pending
// 链接 token 是 32 位随机串，用完即废，验证码只有 6 位，所以要限制 attempts 次数与 15 分钟 TTL
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
	Token  string
	Email  string
	Code   string
	Device entity.DeviceType
	// Ticket 发起注册那个浏览器的 ticket，不匹配就不写密码
	Ticket string
}

type VerifyResult = SessionResult

// VerifyLink 链接验证：token 唯一输入，重发后旧 token 的索引已删除，旧链接立即失效
func (h *VerifyHandler) VerifyLink(ctx context.Context, cmd *VerifyCommand) (*VerifyResult, error) {
	agg, err := h.signupService.VerifyLink(ctx, cmd.Token, cmd.Ticket)
	if err != nil {
		return nil, err
	}

	return h.issueVerifiedSession(ctx, agg, cmd.Device)
}

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
