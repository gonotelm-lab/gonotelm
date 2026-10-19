package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateSourceTitleHandler struct {
	*baseHandler
}

func NewUpdateSourceTitleHandler(
	sourceRepo sourcerepo.Repository,
	notebookAccessService *notebookservice.AccessService,
) *UpdateSourceTitleHandler {
	return &UpdateSourceTitleHandler{
		baseHandler: newBaseHandler(sourceRepo, notebookAccessService),
	}
}

func (h *UpdateSourceTitleHandler) Handle(
	ctx context.Context,
	id valobj.Id,
	title string,
) error {
	targetSource, err := h.handleManage(ctx, id)
	if err != nil {
		return err
	}

	targetSource.UpdateTitle(title)
	if err := h.updateSource(ctx, targetSource); err != nil {
		return errors.WithMessagef(err, "update source title failed, source_id=%s", id)
	}

	return nil
}
