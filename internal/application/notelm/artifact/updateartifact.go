package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
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
	*baseHandler
}

func NewUpdateArtifactHandler(
	repo artifactrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
) *UpdateArtifactHandler {
	return &UpdateArtifactHandler{baseHandler: newBaseHandler(repo, notebookAccessService)}
}

func (h *UpdateArtifactHandler) Handle(ctx context.Context, cmd *UpdateCommand) error {
	a, err := h.checkManageAccess(ctx, cmd.ArtifactId)
	if err != nil {
		return err
	}

	switch cmd.Target {
	case UpdateTargetTitle:
		if err := a.UpdateTitle(cmd.Title); err != nil {
			return err
		}
	default:
		return errors.ErrParams.Msgf("unsupported update target: %s", cmd.Target)
	}

	if err := h.updateArtifact(ctx, a); err != nil {
		return errors.WithMessage(err, "update artifact failed")
	}
	return nil
}
