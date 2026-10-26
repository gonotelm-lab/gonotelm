package pin

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type pinScope int

const (
	pinScopeOwned pinScope = iota
	pinScopeShared
)

type scopeResolver struct {
	notebookRepo notebookrepo.Repository
	memberRepo   notebookrepo.MemberRepository
	pinRepo      pinrepo.Repository
}

func (r *scopeResolver) resolveAccess(
	ctx context.Context,
	actor valobj.Uid,
	notebookId valobj.Id,
) (*notebookentity.Notebook, pinScope, error) {
	nb, err := r.notebookRepo.FindById(ctx, notebookId)
	if err != nil {
		if errors.Is(err, notebookerrors.ErrNotebookNotFound) {
			return nil, 0, notebookerrors.ErrNotebookNotFound
		}
		return nil, 0, errors.WithMessage(err, "find notebook failed")
	}

	if nb.OwnerId == actor {
		return nb, pinScopeOwned, nil
	}

	if _, err := r.memberRepo.FindByNotebookIdAndUserId(ctx, notebookId, actor); err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, 0, notebookerrors.ErrNotebookNotFound
		}
		return nil, 0, errors.WithMessage(err, "find notebook member failed")
	}

	return nb, pinScopeShared, nil
}

// countPinsInScope 统计 actor 在指定 scope 下当前仍有效的置顶数（脏 pin 不计）。
func (r *scopeResolver) countPinsInScope(
	ctx context.Context,
	actor valobj.Uid,
	scope pinScope,
) (int, map[valobj.Id]*pinentity.UserNotebookPin, error) {
	pins, err := r.pinRepo.ListByUserId(ctx, actor)
	if err != nil {
		return 0, nil, errors.WithMessage(err, "list pins failed")
	}
	if len(pins) == 0 {
		return 0, map[valobj.Id]*pinentity.UserNotebookPin{}, nil
	}

	ids := make([]valobj.Id, 0, len(pins))
	pinByNB := make(map[valobj.Id]*pinentity.UserNotebookPin, len(pins))
	for _, p := range pins {
		ids = append(ids, p.NotebookId)
		pinByNB[p.NotebookId] = p
	}

	notebooks, err := r.notebookRepo.ListByIds(ctx, ids)
	if err != nil {
		return 0, nil, errors.WithMessage(err, "list notebooks for pins failed")
	}
	nbById := make(map[valobj.Id]*notebookentity.Notebook, len(notebooks))
	for _, nb := range notebooks {
		nbById[nb.Id] = nb
	}

	var memberSet map[valobj.Id]struct{}
	if scope == pinScopeShared {
		members, err := r.memberRepo.ListByUserId(ctx, actor)
		if err != nil {
			return 0, nil, errors.WithMessage(err, "list members for pin scope failed")
		}
		memberSet = make(map[valobj.Id]struct{}, len(members))
		for _, m := range members {
			memberSet[m.NotebookId] = struct{}{}
		}
	}

	valid := make(map[valobj.Id]*pinentity.UserNotebookPin)
	for _, p := range pins {
		nb := nbById[p.NotebookId]
		if nb == nil {
			continue
		}
		switch scope {
		case pinScopeOwned:
			if nb.OwnerId == actor {
				valid[p.NotebookId] = p
			}
		case pinScopeShared:
			if nb.OwnerId != actor {
				if _, ok := memberSet[p.NotebookId]; ok {
					valid[p.NotebookId] = p
				}
			}
		}
	}

	return len(valid), valid, nil
}
