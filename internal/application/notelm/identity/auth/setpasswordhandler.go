package auth

import (
	"context"
	"log/slog"

	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

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
	OldPassword string
	NewPassword string
}

type SetPasswordResult struct {
	// ReloginRequired 恒为 true：全部会话已作废，前端应引导重新登录
	ReloginRequired bool
}

func (h *SetPasswordHandler) Handle(ctx context.Context, cmd *SetPasswordCommand) (*SetPasswordResult, error) {
	if err := validatePasswordPolicy(cmd.NewPassword); err != nil {
		return nil, err
	}

	userId := pkgcontext.GetUserId(ctx)
	if err := h.userService.SetPassword(ctx, userId, cmd.OldPassword, cmd.NewPassword); err != nil {
		return nil, err
	}

	// 作废全部会话（含当前），失败不回滚密码，按 relogin_required 引导重新登录
	if err := h.sessionRepo.DeleteByUserId(ctx, userId); err != nil {
		slog.ErrorContext(ctx, "revoke sessions after password change failed",
			slog.String("user_id", userId.String()),
			slog.Any("err", err),
		)
	}

	return &SetPasswordResult{ReloginRequired: true}, nil
}
