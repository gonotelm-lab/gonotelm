package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListRequest struct {
	NotebookId valobj.Id
	Limit      int
	Offset     int
}

type ListResponse struct {
	Artifacts []*artifactentity.Artifact
	HasMore   bool
}

type ListArtifactsHandler struct {
	repo     artifactrepo.Repository
	notebook notebookrepo.Repository
}

func NewListArtifactsHandler(repo artifactrepo.Repository, notebook notebookrepo.Repository) *ListArtifactsHandler {
	return &ListArtifactsHandler{repo: repo, notebook: notebook}
}

func (h *ListArtifactsHandler) Handle(ctx context.Context, cmd *ListRequest) (*ListResponse, error) {
	userId := pkgcontext.GetUserId(ctx)
	nb, err := h.notebook.FindById(ctx, cmd.NotebookId)
	if err != nil {
		return nil, err
	}
	if nb.OwnerId != userId {
		return nil, errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", cmd.NotebookId)
	}

	fetchLimit := cmd.Limit + 1
	rows, err := h.repo.ListByNotebookId(ctx, cmd.NotebookId, fetchLimit, cmd.Offset)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > cmd.Limit
	if hasMore {
		rows = rows[:cmd.Limit]
	}
	return &ListResponse{Artifacts: rows, HasMore: hasMore}, nil
}
