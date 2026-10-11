package auth

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// SetPasswordHandler 已认证会话里设置或修改口令。口令策略、旧口令校验与"新旧不能相同"
// 都是领域规则，在 UserService.SetPassword 里；这里只负责调用与改完作废会话。
// 改口令的动机通常是"怀疑账号被别人登着"，所以成功后作废该账号全部会话（含当前），
// 由用户重新登录。
type SetPasswordHandler struct {
	userService *identityservice.UserService
	sessionRepo identrepo.UserSessionRepository
}

func NewSetPasswordHandler(
	userService *identityservice.UserService,
	sessionRepo identrepo.UserSessionRepository,
) *SetPasswordHandler {
	return &SetPasswordHandler{userService: userService, sessionRepo: sessionRepo}
}

type SetPasswordCommand struct {
	UserId      valobj.Uid
	OldPassword string
	NewPassword string
}

type SetPasswordResult struct {
	// ReloginRequired 恒为 true：全部会话已作废，前端应引导重新登录
	ReloginRequired bool
}

func (h *SetPasswordHandler) Handle(ctx context.Context, cmd *SetPasswordCommand) (*SetPasswordResult, error) {
	if err := h.userService.SetPassword(ctx, cmd.UserId, cmd.OldPassword, cmd.NewPassword); err != nil {
		return nil, err
	}

	// 作废全部会话（含当前）。这里失败不回滚口令：若返回错误，用户重试时会拿旧口令再试、
	// 得到"旧口令错误"，反而更糟；按 relogin_required 引导重新登录即可。
	if err := h.sessionRepo.DeleteByUserId(ctx, cmd.UserId); err != nil {
		slog.ErrorContext(ctx, "revoke sessions after password change failed",
			slog.String("user_id", cmd.UserId.String()),
			slog.Any("err", err),
		)
	}

	return &SetPasswordResult{ReloginRequired: true}, nil
}
