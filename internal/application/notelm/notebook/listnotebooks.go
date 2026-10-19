package notebook

import (
	"context"
	"sort"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	idententity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
)

type SortBy int

const (
	SortByCreateTime SortBy = 0
	SortByLastActive SortBy = 1
)

type AccessScope int

const (
	AccessScopeAll AccessScope = iota
	AccessScopeOwned
	AccessScopeShared
)

type ListNotebooksHandler struct {
	*baseHandler
	notebookRepo notebookrepo.Repository
	memberRepo   notebookrepo.MemberRepository
}

func NewListNotebooksHandler(
	notebookRepo notebookrepo.Repository,
	memberRepo notebookrepo.MemberRepository,
	userRepo identrepo.UserRepository,
) *ListNotebooksHandler {
	return &ListNotebooksHandler{
		baseHandler:  newBaseHandler(notebookRepo, nil, userRepo),
		notebookRepo: notebookRepo,
		memberRepo:   memberRepo,
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
	owned, err := h.notebookRepo.ListByOwner(ctx, query.UserId)
	if err != nil {
		return nil, errors.WithMessage(err, "list owned notebooks failed")
	}

	shared, err := h.listShared(ctx, query, owned)
	if err != nil {
		return nil, err
	}

	resultOwned := owned
	if query.Scope == AccessScopeShared {
		resultOwned = nil
	}
	notebooks := mergeNotebooks(resultOwned, shared)
	sortNotebooks(notebooks, query.SortBy)

	total := len(notebooks)
	start := min(query.Offset, total)
	end := min(start+query.Limit, total)

	targetNotebooks := make([]*Notebook, 0, len(notebooks[start:end]))
	// 给notebook挂上owner
	for _, notebook := range notebooks[start:end] {
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
		HasMore:   end < total,
	}, nil
}

func (h *ListNotebooksHandler) listShared(
	ctx context.Context,
	query *ListNotebooksHandleQuery,
	owned []*notebookentity.Notebook,
) ([]*notebookentity.Notebook, error) {
	if query.Scope == AccessScopeOwned {
		return nil, nil
	}

	members, err := h.memberRepo.ListByUserId(ctx, query.UserId)
	if err != nil {
		return nil, errors.WithMessage(err, "list notebook memberships failed")
	}

	ownedIds := make(map[valobj.Id]struct{}, len(owned))
	for _, notebook := range owned {
		ownedIds[notebook.Id] = struct{}{}
	}

	sharedIds := make([]valobj.Id, 0, len(members))
	for _, member := range members {
		if _, ok := ownedIds[member.NotebookId]; ok {
			continue
		}
		sharedIds = append(sharedIds, member.NotebookId)
	}
	sharedIds = pkgslices.Unique(sharedIds)

	if len(sharedIds) == 0 {
		return nil, nil
	}

	return h.notebookRepo.FindByIds(ctx, sharedIds)
}

func mergeNotebooks(
	owned []*notebookentity.Notebook,
	shared []*notebookentity.Notebook,
) []*notebookentity.Notebook {
	result := make([]*notebookentity.Notebook, 0, len(owned)+len(shared))
	seen := make(map[valobj.Id]struct{}, len(owned)+len(shared))

	for _, notebook := range owned {
		if _, ok := seen[notebook.Id]; ok {
			continue
		}
		seen[notebook.Id] = struct{}{}
		result = append(result, notebook)
	}

	for _, notebook := range shared {
		if _, ok := seen[notebook.Id]; ok {
			continue
		}
		seen[notebook.Id] = struct{}{}
		result = append(result, notebook)
	}

	return result
}

func sortNotebooks(notebooks []*notebookentity.Notebook, sortBy SortBy) {
	if sortBy == SortByLastActive {
		sort.SliceStable(notebooks, func(i, j int) bool {
			return notebooks[i].UpdateTime.Value() > notebooks[j].UpdateTime.Value()
		})
		return
	}

	sort.SliceStable(notebooks, func(i, j int) bool {
		return notebooks[i].CreateTime.Value() < notebooks[j].CreateTime.Value()
	})
}
