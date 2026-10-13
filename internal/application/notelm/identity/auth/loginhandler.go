package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// LoginHandler 发起三方登录：取 authorize URL，state 存本次握手
type LoginHandler struct {
	*baseHandler
	registry       identrepo.ProviderRegistry
	oauthStateRepo identrepo.OAuthStateRepository
}

func NewLoginHandler(
	registry identrepo.ProviderRegistry,
	oauthStateRepo identrepo.OAuthStateRepository,
	userService *identityservice.UserService,
	userSessionRepo identrepo.UserSessionRepository,
) *LoginHandler {
	return &LoginHandler{
		baseHandler:    newBaseHandler(userService, userSessionRepo),
		registry:       registry,
		oauthStateRepo: oauthStateRepo,
	}
}

type LoginHandleCommand struct {
	LoginProvider    entity.ProviderType
	LoginFrom        entity.DeviceType
	ReturnTo         string // path-escaped url to redirect after login
	CurrentSessionId string
}

type LoginHandleResult struct {
	Authenticated bool
	ReturnTo      string
	LoginInfo     *entity.ProviderOAuthInfo
}

func (h *LoginHandler) Handle(ctx context.Context, cmd *LoginHandleCommand) (*LoginHandleResult, error) {
	// 已登录则跳过 OAuth，直接返回
	// 用 baseHandler.authenticate 检查登录态，它校验闲置/绝对过期并按需滑动续期
	// 否则失效的会话也会被当成已登录
	if cmd.CurrentSessionId != "" {
		_, err := h.authenticate(ctx, cmd.CurrentSessionId)
		if err == nil {
			return &LoginHandleResult{
				Authenticated: true,
				ReturnTo:      cmd.ReturnTo,
			}, nil
		}
		if !errors.Is(err, identerr.ErrUserSessionNotFound) {
			return nil, errors.WithMessage(err, "check current session failed")
		}
	}

	provider, err := h.registry.GetProvider(ctx, cmd.LoginProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "provide login provider failed")
	}

	loginInfo, err := provider.OAuthInfo(ctx, &entity.OAuthInfoRequest{
		ReturnTo: cmd.ReturnTo,
		Device:   cmd.LoginFrom,
		Mode:     entity.ProviderLoginModeLogin,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "get login info failed")
	}

	if err := h.oauthStateRepo.Save(ctx, loginInfo.ToOAuthState()); err != nil {
		return nil, errors.WithMessage(err, "save login state failed")
	}

	return &LoginHandleResult{
		LoginInfo: loginInfo,
	}, nil
}
