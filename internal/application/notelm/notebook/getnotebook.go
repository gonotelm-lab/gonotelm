package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pinerrors "github.com/gonotelm-lab/gonotelm/internal/domain/pin/errors"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetNotebookHandler struct {
	*baseHandler
	pinRepo pinrepo.Repository
}

func NewGetNotebookHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectPublicURLer,
	pinRepo pinrepo.Repository,
) *GetNotebookHandler {
	return &GetNotebookHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo, objectStore),
		pinRepo:     pinRepo,
	}
}

func (h *GetNotebookHandler) Handle(
	ctx context.Context,
	id valobj.Id,
) (*Notebook, error) {
	return withNotebookAccess(h.baseHandler, ctx, id, accessView, h.getNotebook)
}

func (h *GetNotebookHandler) getNotebook(
	ctx context.Context,
	access *notebookentity.Access,
) (*Notebook, error) {
	result := &Notebook{
		Notebook: access.Notebook,
	}
	if err := h.populateNotebooks(ctx, []*Notebook{result}); err != nil {
		return nil, pkgerr.WithMessagef(err, "attach notebook owners failed, notebook_id=%s", access.Notebook.Id)
	}

	actor := pkgcontext.GetUserId(ctx)
	pin, err := h.pinRepo.Find(ctx, actor, access.Notebook.Id)
	if err != nil {
		if !pkgerr.Is(err, pinerrors.ErrPinNotFound) {
			return nil, pkgerr.WithMessage(err, "find pin failed")
		}
	} else {
		result.IsPinned = true
		result.PinnedAt = pin.PinnedAt.Value()
	}

	return result, nil
}
