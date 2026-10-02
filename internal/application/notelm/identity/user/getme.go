package user

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type GetMeHandler struct {
	userRepo    repository.UserRepository
	objectStore adapter.ObjectPublicURLer
}

func NewGetMeHandler(
	userRepo repository.UserRepository,
	objectStore adapter.ObjectPublicURLer,
) *GetMeHandler {
	return &GetMeHandler{userRepo: userRepo, objectStore: objectStore}
}

type MeResult struct {
	UserId    string
	Nickname  string
	AvatarUrl string
}

func (h *GetMeHandler) Handle(ctx context.Context) (*MeResult, error) {
	user, err := h.userRepo.GetById(ctx, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, err
	}

	return &MeResult{
		UserId:    user.Id.String(),
		Nickname:  user.Nickname,
		AvatarUrl: h.avatarURL(ctx, user.Avatar),
	}, nil
}

func (h *GetMeHandler) avatarURL(ctx context.Context, avatar valobj.StoreKey) string {
	if !avatar.Valid() {
		return ""
	}

	url, err := h.objectStore.PublicURL(ctx, avatar)
	if err != nil {
		slog.WarnContext(ctx, "resolve avatar public url failed",
			slog.String("avatar", avatar.String()),
			slog.Any("err", err),
		)
		return ""
	}

	return url
}
