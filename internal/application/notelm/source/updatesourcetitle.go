package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
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
	_, err := withSourceAccess(h.baseHandler, ctx, id, accessManage,
		func(ctx context.Context, source *sourceentity.Source) (struct{}, error) {
			return h.updateTitle(ctx, source, title)
		},
	)
	return err
}

func (h *UpdateSourceTitleHandler) updateTitle(
	ctx context.Context,
	targetSource *sourceentity.Source,
	title string,
) (struct{}, error) {
	targetSource.UpdateTitle(title)
	if err := h.updateSource(ctx, targetSource); err != nil {
		return struct{}{}, errors.WithMessagef(err, "update source title failed, source_id=%s", targetSource.Id)
	}

	return struct{}{}, nil
}
