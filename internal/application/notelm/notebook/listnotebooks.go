package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	idententity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
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
}

func NewListNotebooksHandler(
	notebookRepo notebookrepo.Repository,
	userRepo identrepo.UserRepository,
) *ListNotebooksHandler {
	return &ListNotebooksHandler{
		baseHandler:  newBaseHandler(notebookRepo, nil, userRepo),
		notebookRepo: notebookRepo,
	}
}

type ListNotebooksHandleQuery struct {
	UserId valobj.Uid
	Scope  AccessScope
	Limit  int
	Offset int
	SortBy SortBy
}

type Notebook struct {
	*notebookentity.Notebook
	Owner *idententity.User
}

type ListNotebooksHandleResult struct {
	// Notebooks []*notebookentity.Notebook
	Notebooks []*Notebook
	Limit     int
	Offset    int
	HasMore   bool
}

func (h *ListNotebooksHandler) Handle(
	ctx context.Context,
	query *ListNotebooksHandleQuery,
) (*ListNotebooksHandleResult, error) {
	// 多取一条用于判断 has_more，分页本身由数据库完成。
	spec := &notebookrepo.ListSpec{
		Limit:  query.Limit + 1,
		Offset: query.Offset,
		Order:  query.SortBy.toListOrder(),
	}

	var (
		notebooks []*notebookentity.Notebook
		err       error
	)
	switch query.Scope {
	case AccessScopeOwned:
		notebooks, err = h.notebookRepo.ListByOwner(ctx, query.UserId, spec)
	case AccessScopeShared:
		notebooks, err = h.notebookRepo.ListShared(ctx, query.UserId, spec)
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

	targetNotebooks := make([]*Notebook, 0, len(notebooks))
	// 给notebook挂上owner
	for _, notebook := range notebooks {
		targetNotebooks = append(targetNotebooks, &Notebook{
			Notebook: notebook,
		})
	}
	if err := h.attachNotebookOwners(ctx, targetNotebooks); err != nil {
		return nil, errors.WithMessage(err, "list attach notebook owners failed")
	}

	return &ListNotebooksHandleResult{
		Notebooks: targetNotebooks,
		Limit:     query.Limit,
		Offset:    query.Offset,
		HasMore:   hasMore,
	}, nil
}
