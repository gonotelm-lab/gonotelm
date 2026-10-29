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

// 列出用户的置顶 notebook
func (h *ListPinnedNotebooksHandler) Handle(
	ctx context.Context,
) (*ListPinnedNotebooksHandleResult, error) {
	userId := pkgcontext.GetUserId(ctx)

	// 每用户 pin 上限有限，可全量取
	pins, err := h.pinRepo.ListByUserId(ctx, userId)
	if err != nil {
		return nil, errors.WithMessage(err, "list pins failed")
	}
	if len(pins) == 0 {
		return &ListPinnedNotebooksHandleResult{Notebooks: []*Notebook{}}, nil
	}

	ids := make([]valobj.Id, 0, len(pins))
	for _, p := range pins {
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

	memberSet, err := h.memberNotebookSet(ctx, userId)
	if err != nil {
		return nil, err
	}

	// 过滤已丢失权限的 pin；顺序保持 pinned_at DESC
	target := make([]*Notebook, 0, len(pins))
	for _, p := range pins {
		nb := nbById[p.NotebookId]
		if nb == nil {
			continue
		}
		_, isMember := memberSet[p.NotebookId]
		if !nb.CanPinBy(userId, isMember) {
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

func (h *ListPinnedNotebooksHandler) memberNotebookSet(
	ctx context.Context,
	userId valobj.Uid,
) (map[valobj.Id]struct{}, error) {
	members, err := h.memberRepo.ListByUserId(ctx, userId)
	if err != nil {
		return nil, errors.WithMessage(err, "list members for pin filter failed")
	}
	memberSet := make(map[valobj.Id]struct{}, len(members))
	for _, m := range members {
		memberSet[m.NotebookId] = struct{}{}
	}
	return memberSet, nil
}
