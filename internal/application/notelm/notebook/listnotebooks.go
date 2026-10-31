package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SortBy int

const (
	SortByCreateTime SortBy = 0
	SortByLastActive SortBy = 1
)

func (s SortBy) toListOrder() notebookrepo.ListOrder {
	if s == SortByLastActive {
		return notebookrepo.ListOrderLastActive
	}

	return notebookrepo.ListOrderCreateTime
}

// AccessScope 决定列表查哪一边：owned 与 shared 互斥，不再合并。
type AccessScope int

const (
	AccessScopeOwned AccessScope = iota
	AccessScopeShared
)

type ListNotebooksHandler struct {
	*baseHandler
	notebookRepo notebookrepo.Repository
	pinRepo      pinrepo.Repository
}

func NewListNotebooksHandler(
	notebookRepo notebookrepo.Repository,
	pinRepo pinrepo.Repository,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectPublicURLer,
) *ListNotebooksHandler {
	return &ListNotebooksHandler{
		baseHandler:  newBaseHandler(notebookRepo, nil, userRepo, objectStore),
		notebookRepo: notebookRepo,
		pinRepo:      pinRepo,
	}
}

type ListNotebooksHandleQuery struct {
	Scope  AccessScope
	Limit  int
	Offset int
	SortBy SortBy
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

	excludeIds, err := h.getPinnedNotebookIds(ctx, userId)
	if err != nil {
		return nil, err
	}

	spec := &notebookrepo.ListSpec{
		Limit:  query.Limit + 1,
		Offset: query.Offset,
		Order:  query.SortBy.toListOrder(),
	}

	var notebooks []*notebookentity.Notebook
	switch query.Scope {
	case AccessScopeOwned:
		notebooks, err = h.notebookRepo.ListByOwner(ctx, userId, spec, excludeIds)
	case AccessScopeShared:
		notebooks, err = h.notebookRepo.ListShared(ctx, userId, spec, excludeIds)
	default:
		return nil, errors.ErrParams.Msgf("invalid scope: %d", query.Scope)
	}
	if err != nil {
		return nil, errors.WithMessage(err, "list notebooks failed")
	}

	hasMore := len(notebooks) > query.Limit
	if hasMore {
		notebooks = notebooks[:query.Limit]
	}

	target := make([]*Notebook, 0, len(notebooks))
	for _, nb := range notebooks {
		target = append(target, &Notebook{Notebook: nb})
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

// 获取userId置顶的notebook ids
func (h *ListNotebooksHandler) getPinnedNotebookIds(
	ctx context.Context,
	userId valobj.Uid,
) ([]valobj.Id, error) {
	pins, err := h.pinRepo.ListByUserId(ctx, userId)
	if err != nil {
		return nil, errors.WithMessage(err, "list pins failed")
	}
	if len(pins) == 0 {
		return nil, nil
	}
	ids := make([]valobj.Id, 0, len(pins))
	for _, p := range pins {
		ids = append(ids, p.NotebookId)
	}

	return ids, nil
}
