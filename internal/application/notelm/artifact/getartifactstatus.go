package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
)

type StatusRequest struct{ ArtifactId valobj.Id }

type StatusResponse struct {
	Status     artifactentity.Status
	Title      string
	Result     []byte
	ResultKind artifactentity.ResultKind
	ContentUrl string
	MimeType   string
	CreatedAt  valobj.Time
	UpdatedAt  valobj.Time
}

type GetArtifactStatusHandler struct {
	*baseHandler
	storage adapter.ObjectStore
}

func NewGetArtifactStatusHandler(
	repo artifactrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	storage adapter.ObjectStore,
) *GetArtifactStatusHandler {
	return &GetArtifactStatusHandler{baseHandler: newBaseHandler(repo, notebookAccessService), storage: storage}
}

func (h *GetArtifactStatusHandler) Handle(ctx context.Context, cmd *StatusRequest) (*StatusResponse, error) {
	return withArtifactAccess(h.baseHandler, ctx, cmd.ArtifactId, accessView, h.getStatus)
}

func (h *GetArtifactStatusHandler) getStatus(
	ctx context.Context,
	artifact *artifactentity.Artifact,
) (*StatusResponse, error) {
	resp := &StatusResponse{
		Status:     artifact.Status,
		Title:      artifact.Title,
		Result:     artifact.Result,
		ResultKind: artifact.ResultKind,
		CreatedAt:  artifact.CreateTime,
		UpdatedAt:  artifact.UpdateTime,
	}
	if artifact.ResultKind.Storage() && len(artifact.Result) > 0 {
		resp.ContentUrl, resp.MimeType = materializeStorageResult(ctx, h.storage, artifact.Result)
	}
	return resp, nil
}

// GetView 返回带 ContentUrl 的产物视图，供详情接口直接透传。
func (h *GetArtifactStatusHandler) GetView(ctx context.Context, artifactId valobj.Id) (*ArtifactView, error) {
	return withArtifactAccess(h.baseHandler, ctx, artifactId, accessView, func(ctx context.Context, a *artifactentity.Artifact) (*ArtifactView, error) {
		return toArtifactView(ctx, h.storage, a), nil
	})
}
