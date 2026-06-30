package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateSourceTitleHandler struct {
	sourceRepo sourcerepo.Repository
}

func NewUpdateSourceTitleHandler(sourceRepo sourcerepo.Repository) *UpdateSourceTitleHandler {
	return &UpdateSourceTitleHandler{
		sourceRepo: sourceRepo,
	}
}

func (h *UpdateSourceTitleHandler) Handle(
	ctx context.Context,
	id valobj.Id,
	title string,
) error {
	targetSource, err := h.sourceRepo.FindById(ctx, id)
	if err != nil {
		return errors.WithMessagef(err, "find source failed, source_id=%s", id)
	}

	targetSource.UpdateTitle(title)
	err = h.sourceRepo.Save(ctx, targetSource)
	if err != nil {
		return errors.WithMessagef(err, "update source title failed, source_id=%s", id)
	}

	return nil
}
