package favourite

import (
	"context"
	"log/slog"

	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/storekey"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	favouriterepo "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	idententity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
)

type ListNotebooksHandler struct {
	*baseHandler
	favouriteNotebookRepo favouriterepo.NotebookRepository
	notebookRepo          notebookrepo.Repository
	notebookAccessService *notebookservice.AccessService
	userRepo              identrepo.UserRepository
	resolver              *storekey.Resolver
}

func NewListNotebooksHandler(
	favouriteRepo favouriterepo.Repository,
	favouriteNotebookRepo favouriterepo.NotebookRepository,
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
	objectPublicUrler adapter.ObjectPublicURLer,
) *ListNotebooksHandler {
	return &ListNotebooksHandler{
		baseHandler:           newBaseHandler(favouriteRepo),
		favouriteNotebookRepo: favouriteNotebookRepo,
		notebookRepo:          notebookRepo,
		notebookAccessService: notebookAccessService,
		userRepo:              userRepo,
		resolver:              storekey.New(objectPublicUrler),
	}
}

type ListNotebooksResult struct {
	Items   []*FavouriteNotebookItem
	Limit   int
	Offset  int
	HasMore bool
}

func (h *ListNotebooksHandler) Handle(
	ctx context.Context,
	favouriteId valobj.Id,
	limit, offset int,
) (*ListNotebooksResult, error) {
	if _, err := h.loadOwnedFavourite(ctx, favouriteId); err != nil {
		return nil, err
	}

	spec := &favouriterepo.PageListSpec{Limit: limit + 1, Offset: offset}
	rows, err := h.favouriteNotebookRepo.PageListByFavouriteId(ctx, favouriteId, spec)
	if err != nil {
		return nil, errors.WithMessage(err, "list favourite notebooks failed")
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	if len(rows) == 0 {
		return &ListNotebooksResult{Items: []*FavouriteNotebookItem{}, Limit: limit, Offset: offset}, nil
	}

	ids := make([]valobj.Id, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.NotebookId)
	}
	notebooks, err := h.notebookRepo.ListByIds(ctx, ids)
	if err != nil {
		return nil, errors.WithMessage(err, "list notebooks by ids failed")
	}
	nbById := make(map[valobj.Id]*notebookentity.Notebook, len(notebooks))
	for _, nb := range notebooks {
		nbById[nb.Id] = nb
	}

	actor := pkgcontext.GetUserId(ctx)
	items := make([]*FavouriteNotebookItem, 0, len(rows))
	validViews := make([]*notebookapp.Notebook, 0)

	for _, r := range rows {
		item := &FavouriteNotebookItem{
			NotebookId:   r.NotebookId,
			FavouritedAt: r.FavouritedAt.Value(),
			Invalid:      true,
		}
		nb := nbById[r.NotebookId]
		if nb == nil {
			items = append(items, item)
			continue
		}
		access, err := h.notebookAccessService.Resolve(ctx, r.NotebookId, actor)
		if err != nil {
			slog.WarnContext(ctx, "resolve favourite notebook access failed",
				slog.String("notebook_id", r.NotebookId.String()),
				slog.Any("err", err),
			)
			items = append(items, item)
			continue
		}
		if !access.CanView() {
			items = append(items, item)
			continue
		}
		view := &notebookapp.Notebook{Notebook: nb}
		item.Invalid = false
		item.Notebook = view
		validViews = append(validViews, view)
		items = append(items, item)
	}

	if err := h.populateNotebookViews(ctx, validViews); err != nil {
		return nil, errors.WithMessage(err, "populate notebook views failed")
	}

	return &ListNotebooksResult{
		Items:   items,
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	}, nil
}

func (h *ListNotebooksHandler) populateNotebookViews(
	ctx context.Context,
	notebooks []*notebookapp.Notebook,
) error {
	if len(notebooks) == 0 {
		return nil
	}
	ownerIds := make([]valobj.Uid, 0, len(notebooks))
	for _, n := range notebooks {
		ownerIds = append(ownerIds, n.OwnerId)
	}
	ownerIds = pkgslices.Unique(ownerIds)
	users, err := h.userRepo.BatchGetById(ctx, ownerIds)
	if err != nil {
		return errors.WithMessage(err, "batch get users failed")
	}
	userMap := pkgslices.AsMapF(users, func(t *idententity.User) valobj.Uid { return t.Id })
	for _, n := range notebooks {
		if n.Cover.Valid() {
			n.CoverUrl = h.resolver.ResolvePublic(ctx, n.Cover)
		}
		if owner, ok := userMap[n.OwnerId]; ok && owner != nil {
			n.Owner = &notebookapp.NotebookOwner{
				Id:        owner.Id,
				Nickname:  owner.Nickname,
				AvatarURL: h.resolver.ResolvePublic(ctx, owner.Avatar),
			}
		}
	}
	return nil
}
