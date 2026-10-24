package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type accessKind int

const (
	accessView accessKind = iota
	accessManage
)

type baseHandler struct {
	repo                  artifactrepo.Repository
	notebookAccessService *notebookservice.AccessService
}

func newBaseHandler(
	repo artifactrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
) *baseHandler {
	return &baseHandler{
		repo:                  repo,
		notebookAccessService: notebookAccessService,
	}
}

// withArtifactAccess 鉴权通过后把 artifact 注入业务方法。
func withArtifactAccess[T any](
	h *baseHandler,
	ctx context.Context,
	artifactId valobj.Id,
	kind accessKind,
	fn func(ctx context.Context, artifact *artifactentity.Artifact) (T, error),
) (T, error) {
	artifact, err := h.resolveAccess(ctx, artifactId, kind)
	if err != nil {
		var zero T
		return zero, err
	}
	return fn(ctx, artifact)
}

func (h *baseHandler) resolveAccess(
	ctx context.Context,
	artifactId valobj.Id,
	kind accessKind,
) (*artifactentity.Artifact, error) {
	artifact, err := h.repo.FindById(ctx, artifactId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get artifact failed, artifact_id=%s", artifactId)
	}

	access, err := h.notebookAccessService.Resolve(ctx, artifact.NotebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "resolve notebook access failed, artifact_id=%s", artifactId)
	}

	if err := access.CanViewReturnErr(); err != nil {
		return nil, err
	}

	switch kind {
	case accessView:
		return artifact, nil
	case accessManage:
		if !access.CanManageContent() {
			return nil, errors.ErrPermission.Msgf("artifact write denied, artifact_id=%s", artifactId)
		}
		return artifact, nil
	default:
		return nil, errors.ErrInner.Msgf("unknown artifact access kind=%d", kind)
	}
}

func (h *baseHandler) updateArtifact(ctx context.Context, artifact *artifactentity.Artifact) error {
	affected, err := h.repo.UpdateExisting(ctx, artifact)
	if err != nil {
		return err
	}
	if affected == 0 {
		return artifacterrors.ErrArtifactNotFound
	}

	return nil
}
