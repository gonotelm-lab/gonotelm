package auth

import (
	"context"

	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
)

// AuthHandler 是 interfaces 层使用的会话认证入口,
// 会话能力来自公共的 baseHandler。
type AuthHandler struct {
	*baseHandler
}

func NewAuthHandler(
	userRepo identrepo.UserRepository,
	userSessionRepo identrepo.UserSessionRepository,
) *AuthHandler {
	return &AuthHandler{baseHandler: newBaseHandler(userRepo, userSessionRepo)}
}

func (h *AuthHandler) Authenticate(ctx context.Context, sessionId string) (*AuthenticateResult, error) {
	return h.authenticate(ctx, sessionId)
}

func (h *AuthHandler) SignOut(ctx context.Context, sessionId string) error {
	return h.signOut(ctx, sessionId)
}
