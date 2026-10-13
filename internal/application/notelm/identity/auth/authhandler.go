package auth

import (
	"context"

	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// AuthHandler 是 interfaces 层用的会话认证入口
type AuthHandler struct {
	*baseHandler
}

func NewAuthHandler(
	userService *identityservice.UserService,
	userSessionRepo identrepo.UserSessionRepository,
) *AuthHandler {
	return &AuthHandler{baseHandler: newBaseHandler(userService, userSessionRepo)}
}

func (h *AuthHandler) Authenticate(ctx context.Context, sessionId string) (*AuthenticateResult, error) {
	return h.authenticate(ctx, sessionId)
}

func (h *AuthHandler) SignOut(ctx context.Context, sessionId string) error {
	return h.signOut(ctx, sessionId)
}
