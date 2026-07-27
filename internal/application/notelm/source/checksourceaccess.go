package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CheckSourceAccessHandler struct {
	sourceRepo sourcerepo.Repository
}

func NewCheckSourceAccessHandler(sourceRepo sourcerepo.Repository) *CheckSourceAccessHandler {
	return &CheckSourceAccessHandler{
		sourceRepo: sourceRepo,
	}
}

func (h *CheckSourceAccessHandler) Handle(ctx context.Context, sourceId valobj.Id) error {
	source, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		return errors.WithMessagef(err, "get source failed, source_id=%s", sourceId)
	}

	userId := pkgcontext.GetUserId(ctx)
	if source.OwnerId != userId {
		return errors.ErrPermission.Msgf("source access denied, source_id=%s", sourceId)
	}

	return nil
}
