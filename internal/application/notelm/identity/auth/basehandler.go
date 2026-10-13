package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// baseHandler auth 包内各 handler 共用的会话认证方法
type baseHandler struct {
	userService     *identityservice.UserService
	userSessionRepo identrepo.UserSessionRepository
}

func newBaseHandler(
	userService *identityservice.UserService,
	userSessionRepo identrepo.UserSessionRepository,
) *baseHandler {
	return &baseHandler{
		userService:     userService,
		userSessionRepo: userSessionRepo,
	}
}

type AuthenticateResult struct {
	UserId      valobj.Uid
	LoginSource string
}

// authenticate 校验会话凭证，凭证为空、会话不存在或已失效统一返回 identerr.ErrUserSessionNotFound
func (h *baseHandler) authenticate(ctx context.Context, sessionId string) (*AuthenticateResult, error) {
	if sessionId == "" {
		return nil, identerr.ErrUserSessionNotFound
	}

	session, err := h.userSessionRepo.Get(ctx, sessionId)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get user session")
	}

	now := time.Now()
	if session.IsExpired(now) {
		return nil, identerr.ErrUserSessionNotFound
	}

	// 会话有效不代表用户可用，封禁可能在会话创建之后发生
	user, err := h.userService.GetUser(ctx, session.UserId)
	if err != nil {
		// 用户已被删除，会话一并视为失效，走 401 重新登录
		if errors.Is(err, identerr.ErrUserNotFound) {
			return nil, identerr.ErrUserSessionNotFound
		}
		return nil, errors.WithMessage(err, "failed to get user")
	}
	if user.IsBanned() {
		return nil, identerr.ErrUserBanned
	}

	// 会话有效时按需滑动续期(闲置 UserSessionIdleTimeout,自登录起最长 UserSessionMaxAge);
	// 活跃时延长会话，不越过绝对上限
	// 续期失败不影响本次认证，最坏是会话按旧的过期时间失效
	if session.Touch(now) {
		if err := h.userSessionRepo.Save(ctx, session); err != nil {
			slog.WarnContext(ctx, "renew user session failed, session will expire earlier",
				slog.String("err", err.Error()))
		}
	}

	return &AuthenticateResult{UserId: session.UserId, LoginSource: session.LoginSource}, nil
}

// issueSession 为用户创建新会话并保存
func (h *baseHandler) issueSession(
	ctx context.Context,
	user *identityentity.User,
	device identityentity.DeviceType,
	loginSource string,
) (*identityentity.UserSession, error) {
	session, err := identityentity.NewUserSession(user, device, loginSource)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to new user session")
	}
	if err := h.userSessionRepo.Save(ctx, session); err != nil {
		return nil, errors.WithMessage(err, "failed to save user session")
	}

	return session, nil
}

// signOut 登出，作废会话凭证，凭证为空直接返回成功
func (h *baseHandler) signOut(ctx context.Context, sessionId string) error {
	if sessionId == "" {
		return nil
	}

	if err := h.userSessionRepo.Delete(ctx, sessionId); err != nil {
		return errors.WithMessage(err, "failed to delete user session")
	}

	return nil
}
