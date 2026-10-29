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

type ListPinnedNotebooksHandler struct {
	*baseHandler
	notebookRepo notebookrepo.Repository
	memberRepo   notebookrepo.MemberRepository
	pinRepo      pinrepo.Repository
}

func NewListPinnedNotebooksHandler(
	notebookRepo notebookrepo.Repository,
	memberRepo notebookrepo.MemberRepository,
	pinRepo pinrepo.Repository,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectPublicURLer,
) *ListPinnedNotebooksHandler {
	return &ListPinnedNotebooksHandler{
		baseHandler:  newBaseHandler(notebookRepo, nil, userRepo, objectStore),
		notebookRepo: notebookRepo,
		memberRepo:   memberRepo,
		pinRepo:      pinRepo,
	}
}

type ListPinnedNotebooksHandleResult struct {
	Notebooks []*Notebook
}

// 列出用户的置顶notebook
func (h *ListPinnedNotebooksHandler) Handle(
	ctx context.Context,
) (*ListPinnedNotebooksHandleResult, error) {
	userId := pkgcontext.GetUserId(ctx)

	// 每个用户允许的置顶notebook数量有限 这里可以全量取
	pins, err := h.pinRepo.ListByUserId(ctx, userId)
	if err != nil {
		return nil, errors.WithMessage(err, "list pins failed")
	}
	if len(pins) == 0 {
		return &ListPinnedNotebooksHandleResult{Notebooks: []*Notebook{}}, nil
	}

	validPins, err := h.filterAccessiblePins(ctx, userId, pins)
	if err != nil {
		return nil, err
	}
	if len(validPins) == 0 {
		return &ListPinnedNotebooksHandleResult{Notebooks: []*Notebook{}}, nil
	}

	ids := make([]valobj.Id, 0, len(validPins))
	for _, p := range validPins {
		ids = append(ids, p.NotebookId)
	}
	notebooks, err := h.notebookRepo.ListByIds(ctx, ids)
	if err != nil {
		return nil, errors.WithMessage(err, "list pinned notebooks failed")
	}
	nbById := make(map[valobj.Id]*notebookentity.Notebook, len(notebooks))
	for _, nb := range notebooks {
		nbById[nb.Id] = nb
	}

	target := make([]*Notebook, 0, len(validPins))
	for _, p := range validPins {
		nb := nbById[p.NotebookId]
		if nb == nil {
			continue
		}
		target = append(target, &Notebook{
			Notebook: nb,
			IsPinned: true,
			PinnedAt: p.PinnedAt.Value(),
		})
	}
	if err := h.populateNotebooks(ctx, target); err != nil {
		return nil, errors.WithMessage(err, "list attach notebook owners failed")
	}

	return &ListPinnedNotebooksHandleResult{Notebooks: target}, nil
}

// 获取用户可以访问的置顶的notebook
// 此处过滤被回收掉权限的notebook
func (h *ListPinnedNotebooksHandler) filterAccessiblePins(
	ctx context.Context,
	userId valobj.Uid,
	pins []*pinentity.UserNotebookPin,
) ([]*pinentity.UserNotebookPin, error) {
	ids := make([]valobj.Id, 0, len(pins))
	for _, p := range pins {
		ids = append(ids, p.NotebookId)
	}
	notebooks, err := h.notebookRepo.ListByIds(ctx, ids)
	if err != nil {
		return nil, errors.WithMessage(err, "list notebooks for pins failed")
	}
	nbById := make(map[valobj.Id]*notebookentity.Notebook, len(notebooks))
	for _, nb := range notebooks {
		nbById[nb.Id] = nb
	}

	// 检查是否有权限访问
	members, err := h.memberRepo.ListByUserId(ctx, userId)
	if err != nil {
		return nil, errors.WithMessage(err, "list members for pin filter failed")
	}
	memberSet := make(map[valobj.Id]struct{}, len(members))
	for _, m := range members {
		memberSet[m.NotebookId] = struct{}{}
	}

	out := make([]*pinentity.UserNotebookPin, 0, len(pins))
	for _, p := range pins {
		nb := nbById[p.NotebookId]
		if nb == nil {
			continue
		}
		if nb.OwnerId.EqualsTo(userId) {
			out = append(out, p)
			continue
		}
		if _, ok := memberSet[p.NotebookId]; ok {
			out = append(out, p)
		}
	}

	return out, nil
}
