package pin

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UnpinNotebookHandler struct {
	pinRepo pinrepo.Repository
}

func NewUnpinNotebookHandler(pinRepo pinrepo.Repository) *UnpinNotebookHandler {
	return &UnpinNotebookHandler{pinRepo: pinRepo}
}

func (h *UnpinNotebookHandler) Handle(ctx context.Context, notebookId valobj.Id) (*PinResult, error) {
	actor := pkgcontext.GetUserId(ctx)
	if err := h.pinRepo.Delete(ctx, actor, notebookId); err != nil {
		return nil, errors.WithMessage(err, "delete pin failed")
	}
	return &PinResult{IsPinned: false, PinnedAt: 0}, nil
}
