package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CheckNotebookAccessHandler struct {
	notebookRepo notebookrepo.Repository
}

func NewCheckNotebookAccessHandler(notebookRepo notebookrepo.Repository) *CheckNotebookAccessHandler {
	return &CheckNotebookAccessHandler{
		notebookRepo: notebookRepo,
	}
}

func (h *CheckNotebookAccessHandler) Handle(ctx context.Context, notebookId valobj.Id) error {
	notebook, err := h.notebookRepo.FindById(ctx, notebookId)
	if err != nil {
		return errors.WithMessagef(err, "get notebook failed, notebook_id=%s", notebookId)
	}

	userId := pkgcontext.GetUserId(ctx)
	if notebook.OwnerId != userId {
		return errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", notebookId)
	}

	return nil
}
