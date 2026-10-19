package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
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

func (h *baseHandler) checkViewAccess(
	ctx context.Context,
	artifactId valobj.Id,
) (*artifactentity.Artifact, error) {
	artifact, access, err := h.resolve(ctx, artifactId)
	if err != nil {
		return nil, err
	}

	if err := access.CanViewReturnErr(); err != nil {
		return nil, err
	}

	return artifact, nil
}

func (h *baseHandler) checkManageAccess(
	ctx context.Context,
	artifactId valobj.Id,
) (*artifactentity.Artifact, error) {
	artifact, access, err := h.resolve(ctx, artifactId)
	if err != nil {
		return nil, err
	}

	if err := access.CanViewReturnErr(); err != nil {
		return nil, err
	}

	if !access.CanManageContent() {
		return nil, errors.ErrPermission.Msgf("artifact write denied, artifact_id=%s", artifactId)
	}

	return artifact, nil
}

func (h *baseHandler) resolve(
	ctx context.Context,
	artifactId valobj.Id,
) (*artifactentity.Artifact, *notebookentity.Access, error) {
	artifact, err := h.repo.FindById(ctx, artifactId)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "get artifact failed, artifact_id=%s", artifactId)
	}

	access, err := h.notebookAccessService.Resolve(ctx, artifact.NotebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "resolve notebook access failed, artifact_id=%s", artifactId)
	}

	return artifact, access, nil
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
