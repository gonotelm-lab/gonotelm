package auth

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CallbackHandler struct {
	loginInfoRepo   identrepo.LoginInfoRepository
	userService     *identityservice.UserService
	userSessionRepo identrepo.UserSessionRepository
}

func NewCallbackHandler(
	loginInfoRepo identrepo.LoginInfoRepository,
	userService *identityservice.UserService,
	userSessionRepo identrepo.UserSessionRepository,
) *CallbackHandler {
	return &CallbackHandler{
		loginInfoRepo:   loginInfoRepo,
		userService:     userService,
		userSessionRepo: userSessionRepo,
	}
}

type CallbackHandleCommand struct {
	State            string
	Code             string
	ProviderType     entity.ProviderType
	Device           entity.DeviceType
	CurrentSessionId string
}

func (c *CallbackHandleCommand) validate() error {
	if c.ProviderType == "" {
		return errors.Wrap(errors.ErrInner, "inner error: provider type is required")
	}

	if c.Code == "" {
		return errors.ErrUnauthorized.Msg("code is required")
	}

	return nil
}

type CallbackHandleResult struct {
	SessionId string
	UserId    string
	ReturnTo  string
}

func (h *CallbackHandler) Handle(ctx context.Context, cmd *CallbackHandleCommand) (*CallbackHandleResult, error) {
	if err := cmd.validate(); err != nil {
		return nil, err
	}

	// check state
	loginState, err := h.loginInfoRepo.GetTransientLoginInfo(ctx, cmd.State)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			// state not found or expired
			return nil, domainerr.ErrLoginStateNotFound
		}
		return nil, errors.WithMessage(err, "failed to get login state")
	}

	if loginState.State != cmd.State {
		return nil, domainerr.ErrLoginStateMismatch
	}

	// now state is valid here, delete it
	if err := h.loginInfoRepo.DeleteTransientLoginInfo(ctx, cmd.State); err != nil {
		slog.ErrorContext(ctx, "failed to delete transient login info", slog.Any("err", err))
	}

	// get access token or id token to get user info
	provider, err := h.loginInfoRepo.GetProvider(ctx, cmd.ProviderType)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get provider")
	}

	userInfo, err := provider.GetUserInfo(ctx, cmd.Code, loginState)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get user info")
	}

	user, err := h.userService.GetOrRegister(ctx, identityservice.RegisterParams{
		Provider: cmd.ProviderType,
		Subject:  userInfo.Subject,
		Nickname: userInfo.Name,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get or register user")
	}

	// 登录后轮换会话：先作废旧会话，再签发新会话，防止 session fixation
	if cmd.CurrentSessionId != "" {
		if err := h.userSessionRepo.Delete(ctx, cmd.CurrentSessionId); err != nil {
			return nil, errors.WithMessage(err, "failed to delete previous user session")
		}
	}

	session, err := entity.NewUserSession(user, cmd.Device)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to new user session")
	}
	if err := h.userSessionRepo.Save(ctx, session); err != nil {
		return nil, errors.WithMessage(err, "failed to save user session")
	}

	slog.DebugContext(ctx, "callback handler login ok",
		slog.String("provider_type", string(cmd.ProviderType)),
		slog.String("issuer", userInfo.Issuer),
		slog.String("subject", userInfo.Subject),
		slog.String("user_id", user.Id.String()),
		slog.String("session_id", session.Id),
	)

	return &CallbackHandleResult{
		SessionId: session.Id,
		UserId:    user.Id.String(),
		ReturnTo:  loginState.ReturnTo,
	}, nil
}
