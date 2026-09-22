package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
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
	storage adapter.StorageAdapter
}

func NewGetArtifactStatusHandler(repo artifactrepo.Repository, storage adapter.StorageAdapter) *GetArtifactStatusHandler {
	return &GetArtifactStatusHandler{baseHandler: newBaseHandler(repo), storage: storage}
}

func (h *GetArtifactStatusHandler) Handle(ctx context.Context, cmd *StatusRequest) (*StatusResponse, error) {
	artifact, err := h.handle(ctx, cmd.ArtifactId)
	if err != nil {
		return nil, err
	}

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

func (h *GetArtifactStatusHandler) FindById(ctx context.Context, artifactId valobj.Id) (*artifactentity.Artifact, error) {
	return h.handle(ctx, artifactId)
}

func (h *GetArtifactStatusHandler) AttachStorageURL(ctx context.Context, a *artifactentity.Artifact) (url string, mime string) {
	if a == nil || !a.ResultKind.Storage() || len(a.Result) == 0 {
		return "", ""
	}

	return materializeStorageResult(ctx, h.storage, a.Result)
}
