package favourite

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
	favouriteerrors "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type baseHandler struct {
	favouriteRepo repository.FavouriteRepository
}

func newBaseHandler(favouriteRepo repository.FavouriteRepository) *baseHandler {
	return &baseHandler{favouriteRepo: favouriteRepo}
}

// loadOwnedFavourite 加载属于当前登录用户的收藏夹；不存在或不属于本人 → ErrFavouriteNotFound。
func (h *baseHandler) loadOwnedFavourite(
	ctx context.Context,
	id valobj.Id,
) (*entity.Favourite, error) {
	f, err := h.favouriteRepo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	actor := pkgcontext.GetUserId(ctx)
	if !f.UserId.EqualsTo(actor) {
		return nil, favouriteerrors.ErrFavouriteNotFound
	}
	return f, nil
}

func userFavouriteLockKey(userId valobj.Uid) string {
	return "notelm:user:favourite:lock:" + userId.String()
}

func wrap(err error, msg string) error {
	return errors.WithMessage(err, msg)
}
