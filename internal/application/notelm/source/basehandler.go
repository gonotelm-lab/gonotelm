package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type accessKind int

const (
	accessView accessKind = iota
	accessManage
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

// withSourceAccess 鉴权通过后把 source 注入业务方法。
func withSourceAccess[T any](
	h *baseHandler,
	ctx context.Context,
	sourceId valobj.Id,
	kind accessKind,
	fn func(ctx context.Context, source *sourceentity.Source) (T, error),
) (T, error) {
	source, err := h.resolveAccess(ctx, sourceId, kind)
	if err != nil {
		var zero T
		return zero, err
	}
	return fn(ctx, source)
}

func (h *baseHandler) resolveAccess(
	ctx context.Context,
	sourceId valobj.Id,
	kind accessKind,
) (*sourceentity.Source, error) {
	source, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source failed, source_id=%s", sourceId)
	}

	access, err := h.notebookAccessService.Resolve(ctx, source.NotebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "resolve notebook access failed, source_id=%s", sourceId)
	}

	if err := access.CanViewReturnErr(); err != nil {
		return nil, err
	}

	switch kind {
	case accessView:
		return source, nil
	case accessManage:
		if !access.CanManageContent() {
			return nil, errors.ErrPermission.Msgf("source write denied, source_id=%s", sourceId)
		}
		return source, nil
	default:
		return nil, errors.ErrInner.Msgf("unknown source access kind=%d", kind)
	}
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
