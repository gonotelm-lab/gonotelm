package pin

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
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

// resolveAccess 只认 owner / 显式成员；不走 AccessService（避免 public viewer 误放行）。
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
