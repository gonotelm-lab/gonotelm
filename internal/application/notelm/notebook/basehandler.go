package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type baseHandler struct {
	notebookRepo notebookrepo.Repository
}

func newBaseHandler(notebookRepo notebookrepo.Repository) *baseHandler {
	return &baseHandler{
		notebookRepo: notebookRepo,
	}
}

// 所有handler都要先处理这个公共的操作
func (h *baseHandler) handle(ctx context.Context, notebookId valobj.Id) (*notebookentity.Notebook, error) {
	notebook, err := h.notebookRepo.FindById(ctx, notebookId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", notebookId)
	}

	// check owner id
	userId := pkgcontext.GetUserId(ctx)
	if notebook.OwnerId != userId {
		return nil, errors.WithMessagef(errors.ErrPermission, "notebook access denied, notebook_id=%s", notebookId)
	}

	return notebook, nil
}
