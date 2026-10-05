package user

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgimage "github.com/gonotelm-lab/gonotelm/pkg/image"
)

type UpdateAvatarHandler struct {
	userRepo    repository.UserRepository
	objectStore adapter.ObjectStore
	keyFactory  adapter.StoreKeyFactory
}

func NewUpdateAvatarHandler(
	userRepo repository.UserRepository,
	objectStore adapter.ObjectStore,
	keyFactory adapter.StoreKeyFactory,
) *UpdateAvatarHandler {
	return &UpdateAvatarHandler{
		userRepo:    userRepo,
		objectStore: objectStore,
		keyFactory:  keyFactory,
	}
}

func (h *UpdateAvatarHandler) Handle(ctx context.Context, content []byte) (string, error) {
	contentType, err := pkgimage.Validate(content, pkgimage.Limits{
		MaxBytes:     entity.MaxAvatarBytes,
		MaxDimension: entity.MaxAvatarDimension,
	})
	if err != nil {
		return "", errors.WithMessage(identityerrors.ErrInvalidAvatar, err.Error())
	}

	user, err := h.userRepo.GetById(ctx, pkgcontext.GetUserId(ctx))
	if err != nil {
		return "", err
	}

	oldAvatar := user.Avatar

	newAvatar, err := user.GenerateAvatarKey(h.keyFactory)
	if err != nil {
		return "", errors.WithMessage(err, "create avatar store key failed")
	}
	if err := h.objectStore.Upload(ctx, newAvatar, content, contentType); err != nil {
		return "", errors.WithMessage(err, "upload avatar failed")
	}

	user.SetAvatar(newAvatar)

	if err := h.userRepo.Save(ctx, user); err != nil {
		if delErr := h.objectStore.DeleteObject(ctx, newAvatar); delErr != nil {
			slog.ErrorContext(ctx, "rollback uploaded avatar failed",
				slog.String("user_id", user.Id.String()),
				slog.String("avatar", newAvatar.String()),
				slog.Any("err", delErr),
			)
		}
		return "", errors.WithMessage(err, "save user failed")
	}

	if oldAvatar.Valid() {
		if delErr := h.objectStore.DeleteObject(ctx, oldAvatar); delErr != nil {
			slog.WarnContext(ctx, "delete previous avatar failed",
				slog.String("user_id", user.Id.String()),
				slog.String("avatar", oldAvatar.String()),
				slog.Any("err", delErr),
			)
		}
	}

	url, err := h.objectStore.PublicURL(ctx, newAvatar)
	if err != nil {
		return "", errors.WithMessage(err, "resolve avatar public url failed")
	}

	return url, nil
}
