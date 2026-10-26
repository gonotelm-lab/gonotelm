package pin

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	pinerrors "github.com/gonotelm-lab/gonotelm/internal/domain/pin/errors"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type PinNotebookHandler struct {
	*scopeResolver
}

func NewPinNotebookHandler(
	notebookRepo notebookrepo.Repository,
	memberRepo notebookrepo.MemberRepository,
	pinRepo pinrepo.Repository,
) *PinNotebookHandler {
	return &PinNotebookHandler{
		scopeResolver: &scopeResolver{
			notebookRepo: notebookRepo,
			memberRepo:   memberRepo,
			pinRepo:      pinRepo,
		},
	}
}

type PinResult struct {
	IsPinned bool
	PinnedAt int64
}

func (h *PinNotebookHandler) Handle(ctx context.Context, notebookId valobj.Id) (*PinResult, error) {
	actor := pkgcontext.GetUserId(ctx)

	_, scope, err := h.resolveAccess(ctx, actor, notebookId)
	if err != nil {
		return nil, err
	}

	if existing, err := h.pinRepo.Find(ctx, actor, notebookId); err == nil {
		existing.TouchPinnedAt()
		if err := h.pinRepo.Save(ctx, existing); err != nil {
			return nil, errors.WithMessage(err, "refresh pin failed")
		}
		return &PinResult{IsPinned: true, PinnedAt: existing.PinnedAt.Value()}, nil
	} else if !errors.Is(err, pinerrors.ErrPinNotFound) {
		return nil, errors.WithMessage(err, "find pin failed")
	}

	count, _, err := h.countPinsInScope(ctx, actor, scope)
	if err != nil {
		return nil, err
	}
	if count >= pinentity.MaxPinsPerScope {
		return nil, pinerrors.ErrPinLimitExceeded
	}

	pin, err := pinentity.NewUserNotebookPin(actor, notebookId)
	if err != nil {
		return nil, err
	}
	if err := h.pinRepo.Save(ctx, pin); err != nil {
		return nil, errors.WithMessage(err, "save pin failed")
	}

	return &PinResult{IsPinned: true, PinnedAt: pin.PinnedAt.Value()}, nil
}
