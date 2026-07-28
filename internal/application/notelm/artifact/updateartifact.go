package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateTarget string

const UpdateTargetTitle UpdateTarget = "title"

type UpdateCommand struct {
	ArtifactId valobj.Id
	Target     UpdateTarget
	Title      string
}

type UpdateArtifactHandler struct {
	repo artifactrepo.Repository
}

func NewUpdateArtifactHandler(repo artifactrepo.Repository) *UpdateArtifactHandler {
	return &UpdateArtifactHandler{repo: repo}
}

func (h *UpdateArtifactHandler) Handle(ctx context.Context, cmd *UpdateCommand) error {
	a, err := h.repo.FindById(ctx, cmd.ArtifactId)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}

	switch cmd.Target {
	case UpdateTargetTitle:
		if err := a.UpdateTitle(cmd.Title); err != nil {
			return err
		}
	default:
		return errors.ErrParams.Msgf("unsupported update target: %s", cmd.Target)
	}

	if err := h.repo.Save(ctx, a); err != nil {
		return errors.WithMessage(err, "save artifact failed")
	}
	return nil
}
