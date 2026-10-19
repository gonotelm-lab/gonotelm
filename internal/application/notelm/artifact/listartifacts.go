package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
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
	repo                  artifactrepo.Repository
	notebookAccessService *notebookservice.AccessService
}

func NewListArtifactsHandler(
	repo artifactrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
) *ListArtifactsHandler {
	return &ListArtifactsHandler{repo: repo, notebookAccessService: notebookAccessService}
}

func (h *ListArtifactsHandler) Handle(ctx context.Context, cmd *ListRequest) (*ListResponse, error) {
	access, err := h.notebookAccessService.Resolve(ctx, cmd.NotebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", cmd.NotebookId)
	}
	if err := access.CanViewReturnErr(); err != nil {
		return nil, err
	}

	fetchLimit := cmd.Limit + 1
	rows, err := h.repo.ListByNotebookId(ctx, cmd.NotebookId, &artifactrepo.ListSpec{
		Limit:  fetchLimit,
		Offset: cmd.Offset,
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > cmd.Limit
	if hasMore {
		rows = rows[:cmd.Limit]
	}
	return &ListResponse{Artifacts: rows, HasMore: hasMore}, nil
}
