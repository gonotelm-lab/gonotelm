package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type AuthHandler struct {
	userSessionRepo identrepo.UserSessionRepository
}

func NewAuthHandler(userSessionRepo identrepo.UserSessionRepository) *AuthHandler {
	return &AuthHandler{userSessionRepo: userSessionRepo}
}

type AuthenticateResult struct {
	UserId valobj.Uid
}

// 凭证为空、会话不存在或已失效统一返回 identityerrors.ErrUserSessionNotFound。
func (h *AuthHandler) Authenticate(ctx context.Context, sessionId string) (*AuthenticateResult, error) {
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

// SignOut 作废会话凭证,登出。凭证为空视为已登出,直接成功。
func (h *AuthHandler) SignOut(ctx context.Context, sessionId string) error {
	if sessionId == "" {
		return nil
	}

	if err := h.userSessionRepo.Delete(ctx, sessionId); err != nil {
		return errors.WithMessage(err, "failed to delete user session")
	}

	return nil
}
