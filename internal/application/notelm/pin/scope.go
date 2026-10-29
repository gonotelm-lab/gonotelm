package pin

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
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
) error {
	nb, err := r.notebookRepo.FindById(ctx, notebookId)
	if err != nil {
		if errors.Is(err, notebookerrors.ErrNotebookNotFound) {
			return notebookerrors.ErrNotebookNotFound
		}
		return errors.WithMessage(err, "find notebook failed")
	}

	// 只有 notebook owner 和 notebook member 能够执行 pin，其他都不能执行pin
	if nb.OwnerId.EqualsTo(actor) {
		return nil
	}

	if _, err := r.memberRepo.FindByNotebookIdAndUserId(ctx, notebookId, actor); err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return notebookerrors.ErrNotebookNotFound
		}
		return errors.WithMessage(err, "find notebook member failed")
	}

	return nil
}
