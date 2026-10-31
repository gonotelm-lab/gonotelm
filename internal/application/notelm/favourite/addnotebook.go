package favourite

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
	favouriteerrors "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/service"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type AddNotebookHandler struct {
	*baseHandler
	favouriteNotebookRepo repository.FavouriteNotebookRepository
	favouriteService      *service.Service
	notebookAccessService *notebookservice.AccessService
	distLock              adapter.DistributedLock
}

func NewAddNotebookHandler(
	favouriteRepo repository.FavouriteRepository,
	favouriteNotebookRepo repository.FavouriteNotebookRepository,
	favouriteService *service.Service,
	notebookAccessService *notebookservice.AccessService,
	distLock adapter.DistributedLock,
) *AddNotebookHandler {
	return &AddNotebookHandler{
		baseHandler:           newBaseHandler(favouriteRepo),
		favouriteNotebookRepo: favouriteNotebookRepo,
		favouriteService:      favouriteService,
		notebookAccessService: notebookAccessService,
		distLock:              distLock,
	}
}

func (h *AddNotebookHandler) Handle(
	ctx context.Context,
	favouriteId, notebookId valobj.Id,
) error {
	f, err := h.loadOwnedFavourite(ctx, favouriteId)
	if err != nil {
		return err
	}

	actor := pkgcontext.GetUserId(ctx)
	access, err := h.notebookAccessService.Resolve(ctx, notebookId, actor)
	if err != nil {
		return wrap(err, "resolve notebook access failed")
	}
	if err := access.CanViewReturnErr(); err != nil {
		return notebookerrors.ErrNotebookNotFound
	}

	if _, err := h.favouriteNotebookRepo.Find(ctx, f.Id, notebookId); err == nil {
		return nil // 已在夹中，幂等且不刷新 favourited_at
	} else if !errors.Is(err, favouriteerrors.ErrFavouriteNotebookNotFound) {
		return wrap(err, "find favourite notebook failed")
	}

	if err := h.favouriteService.CheckNotebookLimit(ctx, f.Id); err != nil {
		return err
	}

	return h.distLock.WithLockWithRetry(
		ctx,
		userFavouriteLockKey(actor),
		adapter.DefaultRetrySpec,
		func(ctx context.Context) error {
			if _, err := h.favouriteNotebookRepo.Find(ctx, f.Id, notebookId); err == nil {
				return nil
			} else if !errors.Is(err, favouriteerrors.ErrFavouriteNotebookNotFound) {
				return wrap(err, "find favourite notebook failed")
			}
			if err := h.favouriteService.CheckNotebookLimit(ctx, f.Id); err != nil {
				return err
			}
			row, err := entity.NewFavouriteNotebook(f.Id, notebookId)
			if err != nil {
				return err
			}
			if err := h.favouriteNotebookRepo.Save(ctx, row); err != nil {
				return wrap(err, "save favourite notebook failed")
			}
			return nil
		},
	)
}
