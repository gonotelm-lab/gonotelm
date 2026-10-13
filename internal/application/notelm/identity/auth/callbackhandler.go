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
	*baseHandler
	registry       identrepo.ProviderRegistry
	oauthStateRepo identrepo.OAuthStateRepository
	userService    *identityservice.UserService
}

func NewCallbackHandler(
	registry identrepo.ProviderRegistry,
	oauthStateRepo identrepo.OAuthStateRepository,
	userService *identityservice.UserService,
	userSessionRepo identrepo.UserSessionRepository,
) *CallbackHandler {
	return &CallbackHandler{
		baseHandler:    newBaseHandler(userService, userSessionRepo),
		registry:       registry,
		oauthStateRepo: oauthStateRepo,
		userService:    userService,
	}
}

type CallbackHandleCommand struct {
	State            string
	Code             string
	ProviderType     entity.ProviderType
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
	// Mode 本次回调属于登录还是绑定，登录创建会话，绑定只挂身份
	Mode entity.ProviderLoginMode
}

func (h *CallbackHandler) Handle(ctx context.Context, cmd *CallbackHandleCommand) (*CallbackHandleResult, error) {
	result := &CallbackHandleResult{Mode: entity.ProviderLoginModeLogin}

	if err := cmd.validate(); err != nil {
		return result, err
	}

	// check state
	oauthState, err := h.oauthStateRepo.Get(ctx, cmd.State)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			// state not found or expired
			return result, domainerr.ErrOAuthStateNotFound
		}
		return result, errors.WithMessage(err, "failed to get login state")
	}

	result.ReturnTo = oauthState.ReturnTo
	if oauthState.Mode != "" {
		result.Mode = oauthState.Mode
	}

	if oauthState.State != cmd.State {
		return result, domainerr.ErrOAuthStateMismatch
	}

	// now state is valid here, delete it
	if err := h.oauthStateRepo.Delete(ctx, cmd.State); err != nil {
		slog.ErrorContext(ctx, "failed to delete login state", slog.Any("err", err))
	}

	// get access token or id token to get user info
	provider, err := h.registry.GetProvider(ctx, cmd.ProviderType)
	if err != nil {
		return result, errors.WithMessage(err, "failed to get provider")
	}

	userInfo, err := provider.GetUserInfo(ctx, cmd.Code, oauthState)
	if err != nil {
		return result, errors.WithMessage(err, "failed to get user info")
	}

	if oauthState.Mode.IsBindMode() {
		if err := h.handleBind(ctx, cmd, oauthState, userInfo, result); err != nil {
			return result, err
		}

		return result, nil
	}

	// 先认身份，建新身份时才按邮箱决定挂到哪个账号
	agg, err := h.userService.ResolveOrCreateByProvider(ctx,
		identityservice.ResolveProviderParams{
			Provider:       cmd.ProviderType,
			Subject:        userInfo.Subject,
			Email:          userInfo.Email,
			EmailVerified:  userInfo.EmailVerified,
			Nickname:       userInfo.Name,
			OuterAvatarURL: userInfo.AvatarURL,
		})
	if err != nil {
		return result, errors.WithMessage(err, "resolve user failed")
	}

	// 封禁用户不允许重新登录，否则下一次登录就能绕过封禁
	if agg.IsBanned() {
		return result, domainerr.ErrUserBanned
	}

	// 登录后轮换会话：先作废旧会话，再创建新会话，防止 session fixation
	if err := h.signOut(ctx, cmd.CurrentSessionId); err != nil {
		return result, err
	}

	session, err := h.issueSession(ctx, agg.User(), oauthState.Device, cmd.ProviderType.String())
	if err != nil {
		return result, err
	}

	slog.DebugContext(ctx, "callback handler login ok",
		slog.String("provider_type", string(cmd.ProviderType)),
		slog.String("issuer", userInfo.Issuer),
		slog.String("subject", userInfo.Subject),
		slog.String("user_id", agg.Id().String()),
		slog.String("session_id", session.Id),
		slog.String("avatar", agg.User().Avatar.String()),
		slog.Any("raw", userInfo.Raw),
	)

	result.SessionId = session.Id
	result.UserId = agg.Id().String()

	return result, nil
}

// PeekState 解析 state 对应的流程与回跳地址，不消费 state
func (h *CallbackHandler) PeekState(ctx context.Context, state string) *CallbackHandleResult {
	if state == "" {
		return nil
	}

	oauthState, err := h.oauthStateRepo.Get(ctx, state)
	if err != nil {
		return nil
	}

	return &CallbackHandleResult{Mode: oauthState.Mode, ReturnTo: oauthState.ReturnTo}
}

// handleBind 绑定分支：回调必须来自发起绑定的那个会话，否则就是账户关联 CSRF
// 结果写进调用方的 result，失败时由它带 Mode/ReturnTo 跳转
func (h *CallbackHandler) handleBind(
	ctx context.Context,
	cmd *CallbackHandleCommand,
	oauthState *entity.OAuthState,
	userInfo *entity.ProviderUserInfo,
	result *CallbackHandleResult,
) error {
	if oauthState.SessionId == "" || oauthState.SessionId != cmd.CurrentSessionId {
		return domainerr.ErrOAuthStateMismatch
	}

	authn, err := h.authenticate(ctx, cmd.CurrentSessionId)
	if err != nil {
		return err
	}

	agg, err := h.userService.BindIdentity(ctx, authn.UserId,
		identityservice.ResolveProviderParams{
			Provider:       cmd.ProviderType,
			Subject:        userInfo.Subject,
			Email:          userInfo.Email,
			EmailVerified:  userInfo.EmailVerified,
			Nickname:       userInfo.Name,
			OuterAvatarURL: userInfo.AvatarURL,
		})
	if err != nil {
		return errors.WithMessage(err, "link identity failed")
	}

	slog.InfoContext(ctx, "callback handler bind ok",
		slog.String("provider_type", string(cmd.ProviderType)),
		slog.String("subject", userInfo.Subject),
		slog.String("user_id", agg.Id().String()),
	)

	result.Mode = entity.ProviderLoginModeBind
	result.UserId = agg.Id().String()

	return nil
}
