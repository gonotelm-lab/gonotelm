package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type LoginHandler struct {
	loginRepo       repository.LoginInfoRepository
	userSessionRepo repository.UserSessionRepository
}

func NewLoginHandler(
	loginProviderRepo repository.LoginInfoRepository,
	userSessionRepo repository.UserSessionRepository,
) *LoginHandler {
	return &LoginHandler{
		loginRepo:       loginProviderRepo,
		userSessionRepo: userSessionRepo,
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
	// 已登录则跳过 OAuth，直接返回
	if cmd.CurrentSessionId != "" {
		_, err := h.userSessionRepo.Get(ctx, cmd.CurrentSessionId)
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
