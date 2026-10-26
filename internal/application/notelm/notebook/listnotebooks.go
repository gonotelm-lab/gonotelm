package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SortBy int

const (
	SortByCreateTime SortBy = 0
	SortByLastActive SortBy = 1
)

// toListOrder 把接口层的排序映射成 repository 的排序枚举。
func (s SortBy) toListOrder() notebookrepo.ListOrder {
	if s == SortByLastActive {
		return notebookrepo.ListOrderLastActive
	}

	return notebookrepo.ListOrderCreateTime
}

// AccessScope 决定列表查哪一边：owned 与 shared 互斥，不再合并。
type AccessScope int

const (
	// AccessScopeOwned 只返回当前用户作为 owner 的 notebook。
	AccessScopeOwned AccessScope = iota
	// AccessScopeShared 只返回显式分享给当前用户、且 owner 不是本人的 notebook。
	AccessScopeShared
)

type ListNotebooksHandler struct {
	*baseHandler
	notebookRepo notebookrepo.Repository
	memberRepo   notebookrepo.MemberRepository
	pinRepo      pinrepo.Repository
}

func NewListNotebooksHandler(
	notebookRepo notebookrepo.Repository,
	memberRepo notebookrepo.MemberRepository,
	pinRepo pinrepo.Repository,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectPublicURLer,
) *ListNotebooksHandler {
	return &ListNotebooksHandler{
		baseHandler:  newBaseHandler(notebookRepo, nil, userRepo, objectStore),
		notebookRepo: notebookRepo,
		memberRepo:   memberRepo,
		pinRepo:      pinRepo,
	}
}

type ListNotebooksHandleQuery struct {
	Scope  AccessScope
	Limit  int
	Offset int
	SortBy SortBy
}

// NotebookOwner 应用层对外的 owner 视图，AvatarURL 已是公有读完整 URL。
type NotebookOwner struct {
	Id        valobj.Uid
	Nickname  string
	AvatarURL string
}

type Notebook struct {
	*notebookentity.Notebook
	Owner    *NotebookOwner
	CoverUrl string
	IsPinned bool
	PinnedAt int64
}

type ListNotebooksHandleResult struct {
	Notebooks []*Notebook
	Limit     int
	Offset    int
	HasMore   bool
}

func (h *ListNotebooksHandler) Handle(
	ctx context.Context,
	query *ListNotebooksHandleQuery,
) (*ListNotebooksHandleResult, error) {
	userId := pkgcontext.GetUserId(ctx)

	scopedPins, excludeIds, err := h.scopedPins(ctx, userId, query.Scope)
	if err != nil {
		return nil, err
	}

	nPinned := len(scopedPins)
	var (
		pagePinEntities []*notebookentity.Notebook
		pinMeta         []*pinentity.UserNotebookPin
		needUnpinned    int
		unpinnedOffset  int
	)

	switch {
	case query.Offset < nPinned:
		end := query.Offset + query.Limit
		if end > nPinned {
			end = nPinned
		}
		pinMeta = scopedPins[query.Offset:end]
		needUnpinned = query.Limit - len(pinMeta)
		unpinnedOffset = 0
	default:
		needUnpinned = query.Limit
		unpinnedOffset = query.Offset - nPinned
	}

	pinIds := make([]valobj.Id, 0, len(pinMeta))
	for _, p := range pinMeta {
		pinIds = append(pinIds, p.NotebookId)
	}
	if len(pinIds) > 0 {
		pagePinEntities, err = h.notebookRepo.ListByIds(ctx, pinIds)
		if err != nil {
			return nil, errors.WithMessage(err, "list pinned notebooks failed")
		}
	}
	pinNBById := make(map[valobj.Id]*notebookentity.Notebook, len(pagePinEntities))
	for _, nb := range pagePinEntities {
		pinNBById[nb.Id] = nb
	}

	var unpinned []*notebookentity.Notebook
	hasMore := false
	if needUnpinned > 0 {
		spec := &notebookrepo.ListSpec{
			Limit:      needUnpinned + 1,
			Offset:     unpinnedOffset,
			Order:      query.SortBy.toListOrder(),
			ExcludeIds: excludeIds,
		}
		switch query.Scope {
		case AccessScopeOwned:
			unpinned, err = h.notebookRepo.ListByOwner(ctx, userId, spec)
		case AccessScopeShared:
			unpinned, err = h.notebookRepo.ListShared(ctx, userId, spec)
		default:
			return nil, errors.ErrParams.Msgf("invalid scope: %d", query.Scope)
		}
		if err != nil {
			return nil, errors.WithMessage(err, "list notebooks failed")
		}
		hasMore = len(unpinned) > needUnpinned
		if hasMore {
			unpinned = unpinned[:needUnpinned]
		}
	} else if query.Offset+query.Limit < nPinned {
		hasMore = true
	} else {
		// 本页只含置顶且刚好用尽 limit：还要看非置顶是否还有
		spec := &notebookrepo.ListSpec{
			Limit:      1,
			Offset:     0,
			Order:      query.SortBy.toListOrder(),
			ExcludeIds: excludeIds,
		}
		var probe []*notebookentity.Notebook
		switch query.Scope {
		case AccessScopeOwned:
			probe, err = h.notebookRepo.ListByOwner(ctx, userId, spec)
		case AccessScopeShared:
			probe, err = h.notebookRepo.ListShared(ctx, userId, spec)
		}
		if err != nil {
			return nil, errors.WithMessage(err, "probe unpinned notebooks failed")
		}
		hasMore = len(probe) > 0
	}

	target := make([]*Notebook, 0, len(pinMeta)+len(unpinned))
	for _, p := range pinMeta {
		nb := pinNBById[p.NotebookId]
		if nb == nil {
			continue
		}
		target = append(target, &Notebook{
			Notebook: nb,
			IsPinned: true,
			PinnedAt: p.PinnedAt.Value(),
		})
	}
	for _, nb := range unpinned {
		target = append(target, &Notebook{
			Notebook: nb,
			IsPinned: false,
			PinnedAt: 0,
		})
	}

	if err := h.populateNotebooks(ctx, target); err != nil {
		return nil, errors.WithMessage(err, "list attach notebook owners failed")
	}

	return &ListNotebooksHandleResult{
		Notebooks: target,
		Limit:     query.Limit,
		Offset:    query.Offset,
		HasMore:   hasMore,
	}, nil
}

// scopedPins 返回当前 scope 下有效置顶（pinned_at DESC）及对应 exclude id。
func (h *ListNotebooksHandler) scopedPins(
	ctx context.Context,
	userId valobj.Uid,
	scope AccessScope,
) ([]*pinentity.UserNotebookPin, []valobj.Id, error) {
	pins, err := h.pinRepo.ListByUserId(ctx, userId)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "list pins failed")
	}
	if len(pins) == 0 {
		return nil, nil, nil
	}

	ids := make([]valobj.Id, 0, len(pins))
	for _, p := range pins {
		ids = append(ids, p.NotebookId)
	}
	notebooks, err := h.notebookRepo.ListByIds(ctx, ids)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "list notebooks for pins failed")
	}
	nbById := make(map[valobj.Id]*notebookentity.Notebook, len(notebooks))
	for _, nb := range notebooks {
		nbById[nb.Id] = nb
	}

	var memberSet map[valobj.Id]struct{}
	if scope == AccessScopeShared {
		members, err := h.memberRepo.ListByUserId(ctx, userId)
		if err != nil {
			return nil, nil, errors.WithMessage(err, "list members for pin filter failed")
		}
		memberSet = make(map[valobj.Id]struct{}, len(members))
		for _, m := range members {
			memberSet[m.NotebookId] = struct{}{}
		}
	}

	out := make([]*pinentity.UserNotebookPin, 0, len(pins))
	exclude := make([]valobj.Id, 0, len(pins))
	for _, p := range pins {
		nb := nbById[p.NotebookId]
		if nb == nil {
			continue
		}
		switch scope {
		case AccessScopeOwned:
			if nb.OwnerId != userId {
				continue
			}
		case AccessScopeShared:
			if nb.OwnerId == userId {
				continue
			}
			if _, ok := memberSet[p.NotebookId]; !ok {
				continue
			}
		default:
			continue
		}
		out = append(out, p)
		exclude = append(exclude, p.NotebookId)
	}
	return out, exclude, nil
}
