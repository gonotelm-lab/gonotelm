package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
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
}

func NewListNotebooksHandler(
	notebookRepo notebookrepo.Repository,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectPublicURLer,
) *ListNotebooksHandler {
	return &ListNotebooksHandler{
		baseHandler:  newBaseHandler(notebookRepo, nil, userRepo, objectStore),
		notebookRepo: notebookRepo,
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

	userId := pkgcontext.GetUserId(ctx)

	var (
		notebooks []*notebookentity.Notebook
		err       error
	)
	switch query.Scope {
	case AccessScopeOwned:
		notebooks, err = h.notebookRepo.ListByOwner(ctx, userId, spec)
	case AccessScopeShared:
		notebooks, err = h.notebookRepo.ListShared(ctx, userId, spec)
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
	if err := h.populateNotebooks(ctx, targetNotebooks); err != nil {
		return nil, errors.WithMessage(err, "list attach notebook owners failed")
	}

	return &ListNotebooksHandleResult{
		Notebooks: targetNotebooks,
		Limit:     query.Limit,
		Offset:    query.Offset,
		HasMore:   hasMore,
	}, nil
}
