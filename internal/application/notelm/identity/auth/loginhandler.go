package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type LoginHandler struct {
	*baseHandler
	loginRepo repository.LoginInfoRepository
}

func NewLoginHandler(
	loginProviderRepo repository.LoginInfoRepository,
	userRepo repository.UserRepository,
	userSessionRepo repository.UserSessionRepository,
) *LoginHandler {
	return &LoginHandler{
		baseHandler: newBaseHandler(userRepo, userSessionRepo),
		loginRepo:   loginProviderRepo,
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
	LoginInfo     *entity.ProviderLoginInfo
}

func (h *LoginHandler) Handle(ctx context.Context, cmd *LoginHandleCommand) (*LoginHandleResult, error) {
	// 已登录则跳过 OAuth，直接返回。
	// 复用 baseHandler.authenticate 判定登录态：它负责闲置/绝对过期校验并按需滑动续期，
	// 避免这里的判定比正常请求更宽松（否则已失效的会话仍会被当作已登录）。
	if cmd.CurrentSessionId != "" {
		_, err := h.authenticate(ctx, cmd.CurrentSessionId)
		if err == nil {
			return &LoginHandleResult{
				Authenticated: true,
				ReturnTo:      cmd.ReturnTo,
			}, nil
		}
		if !errors.Is(err, identityerrors.ErrUserSessionNotFound) {
			return nil, errors.WithMessage(err, "check current session failed")
		}
	}

	provider, err := h.loginRepo.GetProvider(ctx, cmd.LoginProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "provide login provider failed")
	}

	request := &entity.LoginInfoRequest{
		ReturnTo: cmd.ReturnTo,
		Device:   cmd.LoginFrom,
	}

	loginInfo, err := provider.LoginInfo(ctx, request)
	if err != nil {
		return nil, errors.WithMessage(err, "get login info failed")
	}

	// store short lived login info
	err = h.loginRepo.SaveTransientLoginInfo(ctx, loginInfo.ToTransient())
	if err != nil {
		return nil, errors.WithMessage(err, "save transient login info failed")
	}

	return &LoginHandleResult{
		LoginInfo: loginInfo,
	}, nil
}
