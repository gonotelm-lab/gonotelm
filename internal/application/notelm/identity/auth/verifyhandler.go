package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// VerifyHandler 收口邮箱验证的两条入口，验证通过后签发会话。
//
//	VerifyLink：邮件链接里的明文 token 即凭据，换出 pending，不需要用户填邮箱。
//	VerifyCode：email + 6 位码换出 pending。
//
// 两者的差别只在"猜测成本"：链接 token 是 32 位随机串、一次性消费，没有爆破面，
// 但泄露一次就等于拿到凭据；验证码只有 6 位，因此必须配合 attempts 上限与 15 分钟 TTL。
// 凭证校验与建号由 SignupService 负责，这里只负责签发会话。
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
	// Ticket 发起注册那个浏览器的票据；不匹配则不写密码
	Ticket string
}

type VerifyResult = SessionResult

// VerifyLink 链接验证：token 唯一输入；重发后旧 token 的索引已被删除，旧链接立即失效。
func (h *VerifyHandler) VerifyLink(ctx context.Context, cmd *VerifyCommand) (*VerifyResult, error) {
	agg, err := h.signupService.VerifyLink(ctx, cmd.Token, cmd.Ticket)
	if err != nil {
		return nil, err
	}

	return h.issueVerifiedSession(ctx, agg, cmd.Device)
}

// VerifyCode 验证码验证：email + 6 位码换出 pending 后建号。
func (h *VerifyHandler) VerifyCode(ctx context.Context, cmd *VerifyCommand) (*VerifyResult, error) {
	agg, err := h.signupService.VerifyCode(ctx, cmd.Email, cmd.Code, cmd.Ticket)
	if err != nil {
		return nil, err
	}

	return h.issueVerifiedSession(ctx, agg, cmd.Device)
}

// issueVerifiedSession 建号成功后签发邮箱来源的会话。
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
