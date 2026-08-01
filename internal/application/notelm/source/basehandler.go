package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type baseHandler struct {
	sourceRepo sourcerepo.Repository
}

func newBaseHandler(sourceRepo sourcerepo.Repository) *baseHandler {
	return &baseHandler{
		sourceRepo: sourceRepo,
	}
}

// 所有handler都要先处理这个公共的操作
func (h *baseHandler) handle(ctx context.Context, sourceId valobj.Id) (*sourceentity.Source, error) {
	source, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source failed, source_id=%s", sourceId)
	}

	// check owner id
	userId := pkgcontext.GetUserId(ctx)
	if source.OwnerId != userId {
		return nil, errors.WithMessagef(errors.ErrPermission, "source access denied, source_id=%s", sourceId)
	}

	return source, nil
}
