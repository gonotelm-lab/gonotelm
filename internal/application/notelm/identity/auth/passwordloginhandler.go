package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// PasswordLoginHandler 邮箱密码登录；失败一律返回统一文案，避免账号枚举。
type PasswordLoginHandler struct {
	*baseHandler
}

func NewPasswordLoginHandler(
	userService *identityservice.UserService,
	userSessionRepo identrepo.UserSessionRepository,
) *PasswordLoginHandler {
	return &PasswordLoginHandler{
		baseHandler: newBaseHandler(userService, userSessionRepo),
	}
}

type PasswordLoginCommand struct {
	Email            string
	Password         string
	Device           entity.DeviceType
	CurrentSessionId string
}

type PasswordLoginResult = SessionResult

func (h *PasswordLoginHandler) Handle(
	ctx context.Context,
	cmd *PasswordLoginCommand,
) (*PasswordLoginResult, error) {
	agg, err := h.userService.GetAccountByEmail(ctx, cmd.Email)
	if err != nil {
		if errors.Is(err, identerr.ErrUserNotFound) {
			return nil, identerr.ErrInvalidCredentials
		}

		return nil, errors.WithMessage(err, "get user failed")
	}

	if err := h.userService.VerifyPassword(ctx, agg.Id(), cmd.Password); err != nil {
		return nil, err
	}

	if agg.IsBanned() {
		return nil, identerr.ErrUserBanned
	}

	if err := h.signOut(ctx, cmd.CurrentSessionId); err != nil {
		return nil, err
	}

	session, err := h.issueSession(ctx, agg.User(), cmd.Device, entity.LoginSourcePassword)
	if err != nil {
		return nil, err
	}

	return &PasswordLoginResult{SessionId: session.Id, UserId: agg.Id().String()}, nil
}
