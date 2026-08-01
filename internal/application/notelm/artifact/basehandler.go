package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type baseHandler struct {
	repo artifactrepo.Repository
}

func newBaseHandler(repo artifactrepo.Repository) *baseHandler {
	return &baseHandler{
		repo: repo,
	}
}

// 所有handler都要先处理这个公共的操作
func (h *baseHandler) handle(ctx context.Context, artifactId valobj.Id) (*artifactentity.Artifact, error) {
	artifact, err := h.repo.FindById(ctx, artifactId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get artifact failed, artifact_id=%s", artifactId)
	}

	// check owner id
	userId := pkgcontext.GetUserId(ctx)
	if !artifact.IsOwner(userId) {
		return nil, errors.WithMessagef(artifacterrors.ErrArtifactNotOwnedByUser,
			"artifact access denied, artifact_id=%s", artifactId)
	}

	return artifact, nil
}
