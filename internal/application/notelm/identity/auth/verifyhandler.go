package auth

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// VerifyHandler 收口邮箱验证的两条入口，最后都走 finishSignup 建号并签发会话。
//
//	VerifyLink：邮件链接里的明文 token 即凭据，换出 pending，不需要用户填邮箱。
//	VerifyCode：email + 6 位码换出 pending。
//
// 两者的差别只在"猜测成本"：链接 token 是 32 位随机串、一次性消费，没有爆破面，
// 但泄露一次就等于拿到凭据；验证码只有 6 位，因此必须配合 attempts 上限与 15 分钟 TTL，
// 限内错码只累加计数，超限直接作废 pending。票据（Ticket）判定两条路径完全一致。
type VerifyHandler struct {
	*baseHandler
	signupRepo  identrepo.SignupRepository
	userService *identityservice.UserService
}

func NewVerifyHandler(
	signupRepo identrepo.SignupRepository,
	userService *identityservice.UserService,
	userRepo identrepo.UserRepository,
	userSessionRepo identrepo.UserSessionRepository,
) *VerifyHandler {
	return &VerifyHandler{
		baseHandler: newBaseHandler(userRepo, userSessionRepo),
		signupRepo:  signupRepo,
		userService: userService,
	}
}

type VerifyCommand struct {
	Token  string
	Email  string
	Code   string
	Device entity.DeviceType
	// Ticket 发起注册那个浏览器的票据；不匹配则不写口令
	Ticket string
}

type VerifyResult = SessionResult

// VerifyLink 链接验证：token 唯一输入；重发后旧 token 的索引已被删除，旧链接立即失效。
func (h *VerifyHandler) VerifyLink(ctx context.Context, cmd *VerifyCommand) (*VerifyResult, error) {
	pending, err := h.signupRepo.GetByLinkToken(ctx, cmd.Token)
	if err != nil {
		return nil, err
	}

	return h.finishSignup(ctx, pending, cmd.Ticket, cmd.Device)
}

// VerifyCode 验证码验证：先按 email 取 pending；码不匹配只累加 attempts，
// 达到 SignupMaxCodeAttempts 就作废整个 pending（用户需重新注册）。
func (h *VerifyHandler) VerifyCode(ctx context.Context, cmd *VerifyCommand) (*VerifyResult, error) {
	email := entity.NormalizeEmail(cmd.Email)

	pending, err := h.signupRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if !pending.MatchesCode(cmd.Code) {
		attempts, incrErr := h.signupRepo.IncrAttempts(ctx, email)
		if incrErr != nil {
			slog.WarnContext(ctx, "incr signup attempts failed", slog.Any("err", incrErr))
		}

		if attempts >= entity.SignupMaxCodeAttempts {
			if delErr := h.signupRepo.DeleteByEmail(ctx, email); delErr != nil {
				slog.WarnContext(ctx, "drop signup pending after too many attempts failed", slog.Any("err", delErr))
			}
			return nil, identerr.ErrVerificationAttemptsExceeded
		}

		return nil, identerr.ErrVerificationCodeMismatch
	}

	return h.finishSignup(ctx, pending, cmd.Ticket, cmd.Device)
}

func (h *VerifyHandler) finishSignup(
	ctx context.Context,
	pending *entity.SignupPending,
	ticket string,
	device entity.DeviceType,
) (*VerifyResult, error) {
	// 只有发起注册的那个浏览器完成验证，才把惰性口令激活
	passwordHash := ""
	if pending.MatchesTicket(ticket) {
		passwordHash = pending.PasswordHash
	}

	user, err := h.userService.CreateVerifiedAccount(ctx, pending.Email, passwordHash)
	if err != nil {
		return nil, err
	}

	if err := h.signupRepo.DeleteByEmail(ctx, pending.Email); err != nil {
		slog.WarnContext(ctx, "delete signup pending failed",
			slog.String("email", pending.Email),
			slog.Any("err", err),
		)
	}

	session, err := h.issueSession(ctx, user, device, entity.LoginSourceEmail)
	if err != nil {
		return nil, err
	}

	return &VerifyResult{SessionId: session.Id, UserId: user.Id.String()}, nil
}
