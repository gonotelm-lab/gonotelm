package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type baseHandler struct {
	sourceRepo            sourcerepo.Repository
	notebookAccessService *notebookservice.AccessService
}

func newBaseHandler(
	sourceRepo sourcerepo.Repository,
	notebookAccessService *notebookservice.AccessService,
) *baseHandler {
	return &baseHandler{
		sourceRepo:            sourceRepo,
		notebookAccessService: notebookAccessService,
	}
}

func (h *baseHandler) handleView(
	ctx context.Context,
	sourceId valobj.Id,
) (*sourceentity.Source, error) {
	source, access, err := h.resolve(ctx, sourceId)
	if err != nil {
		return nil, err
	}

	if !access.CanView() {
		return nil, errors.ErrNoRecord.Msgf("source not found, source_id=%s", sourceId)
	}

	return source, nil
}

func (h *baseHandler) handleManage(
	ctx context.Context,
	sourceId valobj.Id,
) (*sourceentity.Source, error) {
	source, access, err := h.resolve(ctx, sourceId)
	if err != nil {
		return nil, err
	}

	if !access.CanView() {
		return nil, errors.ErrNoRecord.Msgf("source not found, source_id=%s", sourceId)
	}

	if !access.CanManageContent() {
		return nil, errors.ErrPermission.Msgf("source write denied, source_id=%s", sourceId)
	}

	return source, nil
}

func (h *baseHandler) resolve(
	ctx context.Context,
	sourceId valobj.Id,
) (*sourceentity.Source, *notebookentity.Access, error) {
	source, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "get source failed, source_id=%s", sourceId)
	}

	access, err := h.notebookAccessService.Resolve(ctx, source.NotebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "resolve notebook access failed, source_id=%s", sourceId)
	}

	return source, access, nil
}

func (h *baseHandler) updateSource(ctx context.Context, source *sourceentity.Source) error {
	affected, err := h.sourceRepo.UpdateExisting(ctx, source)
	if err != nil {
		return err
	}
	// source记录不存在
	if affected == 0 {
		return sourceerr.ErrSourceNotFound
	}

	return nil
}
