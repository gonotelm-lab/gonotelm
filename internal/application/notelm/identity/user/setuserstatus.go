package user

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/storekey"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SetUserStatusCommand struct {
	UserId string
	Status entity.UserStatus
}

type SetUserStatusHandler struct {
	userService      *identityservice.UserService
	userRepo         identityrepo.UserRepository
	storeKeyResolver *storekey.Resolver
}

func NewSetUserStatusHandler(
	userService *identityservice.UserService,
	userRepo identityrepo.UserRepository,
	objectStore adapter.ObjectPublicURLer,
) *SetUserStatusHandler {
	return &SetUserStatusHandler{
		userService:      userService,
		userRepo:         userRepo,
		storeKeyResolver: storekey.New(objectStore),
	}
}

func (h *SetUserStatusHandler) Handle(ctx context.Context, cmd *SetUserStatusCommand) (*UserView, error) {
	userId, err := valobj.NewUidFromString(cmd.UserId)
	if err != nil {
		return nil, pkgerrors.ErrParams.Msg("invalid user id")
	}

	switch cmd.Status {
	case entity.UserStatusActive, entity.UserStatusBanned:
	default:
		return nil, pkgerrors.ErrParams.Msg("invalid user status")
	}

	user, err := h.userService.GetUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	switch cmd.Status {
	case entity.UserStatusActive:
		user.Activate()
	case entity.UserStatusBanned:
		user.Ban()
	}

	if err := h.userRepo.UpdateStatus(ctx, user); err != nil {
		return nil, err
	}

	agg, err := h.userService.GetAccount(ctx, userId)
	if err != nil {
		return nil, err
	}

	return toUserView(ctx, h.storeKeyResolver, agg), nil
}
