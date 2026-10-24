package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
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
	_, err := withArtifactAccess(h.baseHandler, ctx, cmd.ArtifactId, accessManage,
		func(ctx context.Context, artifact *artifactentity.Artifact) (struct{}, error) {
			return h.update(ctx, artifact, cmd)
		},
	)
	return err
}

func (h *UpdateArtifactHandler) update(
	ctx context.Context,
	a *artifactentity.Artifact,
	cmd *UpdateCommand,
) (struct{}, error) {
	switch cmd.Target {
	case UpdateTargetTitle:
		if err := a.UpdateTitle(cmd.Title); err != nil {
			return struct{}{}, err
		}
	default:
		return struct{}{}, errors.ErrParams.Msgf("unsupported update target: %s", cmd.Target)
	}

	if err := h.updateArtifact(ctx, a); err != nil {
		return struct{}{}, errors.WithMessage(err, "update artifact failed")
	}
	return struct{}{}, nil
}
