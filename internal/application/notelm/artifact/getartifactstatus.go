package artifact

import (
	"context"

	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type StatusRequest struct{ ArtifactId valobj.Id }

type StatusResponse struct {
	Status     artifactentity.Status
	Title      string
	Result     []byte
	ResultKind artifactentity.ResultKind
	ContentUrl string
	MimeType   string
	FlowError  string
}

type GetArtifactStatusHandler struct {
	*baseHandler
	flowc   flow.TaskClient
	storage adapter.StorageAdapter
}

func NewGetArtifactStatusHandler(repo artifactrepo.Repository, flowc flow.TaskClient, storage adapter.StorageAdapter) *GetArtifactStatusHandler {
	return &GetArtifactStatusHandler{baseHandler: newBaseHandler(repo), flowc: flowc, storage: storage}
}

func (h *GetArtifactStatusHandler) Handle(ctx context.Context, cmd *StatusRequest) (*StatusResponse, error) {
	artifact, err := h.handle(ctx, cmd.ArtifactId)
	if err != nil {
		return nil, err
	}

	if artifact.IsTerminal() {
		resp := &StatusResponse{
			Status:     artifact.Status,
			Title:      artifact.Title,
			Result:     artifact.Result,
			ResultKind: artifact.ResultKind,
		}
		if artifact.ResultKind.Storage() && len(artifact.Result) > 0 {
			resp.ContentUrl, resp.MimeType = materializeStorageResult(ctx, h.storage, artifact.Result)
		}
		return resp, nil
	}

	// note 本地异步补 title，不经过 flow
	if artifact.Kind == artifactentity.KindNote {
		return &StatusResponse{Status: artifact.Status}, nil
	}

	if artifact.FlowTaskId == "" {
		return nil, artifacterrors.ErrInvalidFlowTaskId
	}

	info, err := h.flowc.Get(ctx, artifact.FlowTaskId)
	if err != nil {
		return nil, errors.WithMessage(err, "query flow task failed")
	}
	mapped := mapFlowState(info.State)
	return &StatusResponse{Status: mapped, FlowError: string(info.Error)}, nil
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

func mapFlowState(state flowschema.TaskState) artifactentity.Status {
	switch state {
	case flowschema.TaskState_INITED:
		return artifactentity.StatusPending
	case flowschema.TaskState_RUNNING:
		return artifactentity.StatusRunning
	case flowschema.TaskState_DONE:
		return artifactentity.StatusCompleted
	case flowschema.TaskState_FAILED:
		return artifactentity.StatusFailed
	case flowschema.TaskState_CANCELLED:
		return artifactentity.StatusCancelled
	}
	return artifactentity.StatusPending
}
