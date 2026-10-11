package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// PasswordLoginHandler 邮箱密码登录；失败一律返回统一文案，避免账号枚举。
type PasswordLoginHandler struct {
	*baseHandler
	hasher adapter.PasswordHasher
}

func NewPasswordLoginHandler(
	hasher adapter.PasswordHasher,
	userRepo identrepo.UserRepository,
	userSessionRepo identrepo.UserSessionRepository,
) *PasswordLoginHandler {
	return &PasswordLoginHandler{
		baseHandler: newBaseHandler(userRepo, userSessionRepo),
		hasher:      hasher,
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
	email := entity.NormalizeEmail(cmd.Email)

	user, err := h.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, identerr.ErrUserNotFound) {
			return nil, identerr.ErrInvalidCredentials
		}

		return nil, errors.WithMessage(err, "get user failed")
	}

	hash, err := h.userRepo.GetPasswordHash(ctx, user.Id)
	if err != nil {
		return nil, errors.WithMessage(err, "get password hash failed")
	}
	// 无口令（如三方首次登录建的号）与口令错误同样处理
	if hash == "" {
		return nil, identerr.ErrInvalidCredentials
	}
	if err := h.hasher.Verify(ctx, hash, cmd.Password); err != nil {
		return nil, identerr.ErrInvalidCredentials
	}

	if user.IsBanned() {
		return nil, identerr.ErrUserBanned
	}

	if err := h.signOut(ctx, cmd.CurrentSessionId); err != nil {
		return nil, err
	}

	session, err := h.issueSession(ctx, user, cmd.Device, entity.LoginSourcePassword)
	if err != nil {
		return nil, err
	}

	return &PasswordLoginResult{SessionId: session.Id, UserId: user.Id.String()}, nil
}
