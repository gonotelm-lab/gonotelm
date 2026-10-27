package pin

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UnpinNotebookHandler struct {
	*scopeResolver
}

func NewUnpinNotebookHandler(
	notebookRepo notebookrepo.Repository,
	memberRepo notebookrepo.MemberRepository,
	pinRepo pinrepo.Repository,
) *UnpinNotebookHandler {
	return &UnpinNotebookHandler{
		scopeResolver: &scopeResolver{
			notebookRepo: notebookRepo,
			memberRepo:   memberRepo,
			pinRepo:      pinRepo,
		},
	}
}

func (h *UnpinNotebookHandler) Handle(ctx context.Context, notebookId valobj.Id) (*PinResult, error) {
	actor := pkgcontext.GetUserId(ctx)

	if _, _, err := h.resolveAccess(ctx, actor, notebookId); err != nil {
		return nil, err
	}

	if err := h.pinRepo.Delete(ctx, actor, notebookId); err != nil {
		return nil, errors.WithMessage(err, "delete pin failed")
	}
	return &PinResult{IsPinned: false, PinnedAt: 0}, nil
}
