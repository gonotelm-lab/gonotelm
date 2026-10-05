package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// baseHandler 收敛 auth 包内各 handler 共用的会话能力。
// 各 handler 通过嵌入 *baseHandler 复用，而不是互相依赖。
type baseHandler struct {
	userRepo        identrepo.UserRepository
	userSessionRepo identrepo.UserSessionRepository
}

func newBaseHandler(
	userRepo identrepo.UserRepository,
	userSessionRepo identrepo.UserSessionRepository,
) *baseHandler {
	return &baseHandler{
		userRepo:        userRepo,
		userSessionRepo: userSessionRepo,
	}
}

type AuthenticateResult struct {
	UserId valobj.Uid
}

// authenticate 校验会话凭证。凭证为空、会话不存在或已失效统一返回 identityerrors.ErrUserSessionNotFound。
func (h *baseHandler) authenticate(ctx context.Context, sessionId string) (*AuthenticateResult, error) {
	if sessionId == "" {
		return nil, identityerrors.ErrUserSessionNotFound
	}

	session, err := h.userSessionRepo.Get(ctx, sessionId)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get user session")
	}

	now := time.Now()
	if session.IsExpired(now) {
		return nil, identityerrors.ErrUserSessionNotFound
	}

	// 会话有效不代表用户可用：封禁可能发生在会话签发之后，
	user, err := h.userRepo.GetById(ctx, session.UserId)
	if err != nil {
		// 用户已被删除，会话一并视为失效，走 401 重新登录
		if errors.Is(err, identityerrors.ErrUserNotFound) {
			return nil, identityerrors.ErrUserSessionNotFound
		}
		return nil, errors.WithMessage(err, "failed to get user")
	}
	if user.IsBanned() {
		return nil, identityerrors.ErrUserBanned
	}

	// 会话有效时按需滑动续期(闲置 UserSessionIdleTimeout,自登录起最长 UserSessionMaxAge);
	// 活跃则延长会话,但不越过绝对上限。
	// 续期失败不影响本次认证,最差情况是会话按旧的过期时间失效。
	if session.Touch(now) {
		if err := h.userSessionRepo.Save(ctx, session); err != nil {
			slog.WarnContext(ctx, "renew user session failed, session will expire earlier",
				slog.String("err", err.Error()))
		}
	}

	return &AuthenticateResult{UserId: session.UserId}, nil
}

// issueSession 为用户签发新会话并持久化。
func (h *baseHandler) issueSession(
	ctx context.Context,
	user *identityentity.User,
	device identityentity.DeviceType,
) (*identityentity.UserSession, error) {
	session, err := identityentity.NewUserSession(user, device)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to new user session")
	}
	if err := h.userSessionRepo.Save(ctx, session); err != nil {
		return nil, errors.WithMessage(err, "failed to save user session")
	}

	return session, nil
}

// signOut 作废会话凭证,登出。凭证为空视为已登出,直接成功。
func (h *baseHandler) signOut(ctx context.Context, sessionId string) error {
	if sessionId == "" {
		return nil
	}

	if err := h.userSessionRepo.Delete(ctx, sessionId); err != nil {
		return errors.WithMessage(err, "failed to delete user session")
	}

	return nil
}
