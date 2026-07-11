package artifact

import (
	"context"

	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
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
	repo    artifactrepo.Repository
	flowc   flow.TaskClient
	storage StorageGateway
}

func NewGetArtifactStatusHandler(repo artifactrepo.Repository, flowc flow.TaskClient, storage StorageGateway) *GetArtifactStatusHandler {
	return &GetArtifactStatusHandler{repo: repo, flowc: flowc, storage: storage}
}

func (h *GetArtifactStatusHandler) Handle(ctx context.Context, cmd *StatusRequest) (*StatusResponse, error) {
	a, err := h.repo.FindById(ctx, cmd.ArtifactId)
	if err != nil {
		return nil, err
	}
	userId := pkgcontext.GetUserId(ctx)
	if !a.IsOwner(userId) {
		return nil, artifacterrors.ErrArtifactNotOwnedByUser
	}

	if a.IsTerminal() {
		resp := &StatusResponse{Status: a.Status, Title: a.Title, Result: a.Result, ResultKind: a.ResultKind}
		if a.ResultKind.Storage() && len(a.Result) > 0 {
			resp.ContentUrl, resp.MimeType = materializeStorageResult(ctx, h.storage, a.Result)
		}
		return resp, nil
	}

	if a.FlowTaskId == "" {
		return nil, artifacterrors.ErrInvalidFlowTaskId
	}

	info, err := h.flowc.Get(ctx, a.FlowTaskId)
	if err != nil {
		return nil, errors.WithMessage(err, "query flow task failed")
	}
	mapped := mapFlowState(info.State)
	return &StatusResponse{Status: mapped, FlowError: string(info.Error)}, nil
}

func (h *GetArtifactStatusHandler) CheckOwnership(ctx context.Context, artifactId valobj.Id) error {
	a, err := h.repo.FindById(ctx, artifactId)
	if err != nil {
		return err
	}
	userId := pkgcontext.GetUserId(ctx)
	if !a.IsOwner(userId) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	return nil
}

func (h *GetArtifactStatusHandler) FindById(ctx context.Context, artifactId valobj.Id) (*artifactentity.Artifact, error) {
	return h.repo.FindById(ctx, artifactId)
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
