package favourite

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CreateFavouriteHandler struct {
	*baseHandler
	favouriteService *service.Service
	distLock         adapter.DistributedLock
}

func NewCreateFavouriteHandler(
	favouriteRepo repository.FavouriteRepository,
	favouriteService *service.Service,
	distLock adapter.DistributedLock,
) *CreateFavouriteHandler {
	return &CreateFavouriteHandler{
		baseHandler:      newBaseHandler(favouriteRepo),
		favouriteService: favouriteService,
		distLock:         distLock,
	}
}

// 创建新的收藏夹
func (h *CreateFavouriteHandler) Handle(ctx context.Context, name string) (*FavouriteView, error) {
	actor := pkgcontext.GetUserId(ctx)

	if err := h.favouriteService.CheckFavouriteLimit(ctx, actor); err != nil {
		return nil, err
	}

	var view *FavouriteView
	err := h.distLock.WithLockWithRetry(
		ctx,
		userFavouriteLockKey(actor),
		adapter.DefaultRetrySpec,
		h.doHandle(actor, name, &view),
	)
	if err != nil {
		return nil, errors.WithMessage(err, "create favourite with lock failed")
	}
	return view, nil
}

func (h *CreateFavouriteHandler) doHandle(
	actor valobj.Uid,
	name string,
	view **FavouriteView,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := h.favouriteService.CheckFavouriteLimit(ctx, actor); err != nil {
			return err
		}
		f, err := entity.NewFavourite(actor, name)
		if err != nil {
			return err
		}
		if err := h.favouriteRepo.Save(ctx, f); err != nil {
			return errors.WithMessage(err, "save favourite failed")
		}
		*view = &FavouriteView{
			Id:            f.Id,
			Name:          f.Name,
			NotebookCount: 0,
			CreatedAt:     f.CreateTime.Value(),
		}
		return nil
	}
}
