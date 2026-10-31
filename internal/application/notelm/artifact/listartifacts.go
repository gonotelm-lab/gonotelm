package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
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
	Artifacts []*ArtifactView
	HasMore   bool
}

type ListArtifactsHandler struct {
	repo                  artifactrepo.Repository
	notebookAccessService *notebookservice.AccessService
	storage               adapter.ObjectStore
}

func NewListArtifactsHandler(
	repo artifactrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	storage adapter.ObjectStore,
) *ListArtifactsHandler {
	return &ListArtifactsHandler{
		repo:                  repo,
		notebookAccessService: notebookAccessService,
		storage:               storage,
	}
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
	rows, err := h.repo.PageListByNotebookId(ctx, cmd.NotebookId, &artifactrepo.PageListSpec{
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

	views := make([]*ArtifactView, 0, len(rows))
	for _, a := range rows {
		views = append(views, toArtifactView(ctx, h.storage, a))
	}

	return &ListResponse{Artifacts: views, HasMore: hasMore}, nil
}
