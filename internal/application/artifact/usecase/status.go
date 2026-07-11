package usecase

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
	FlowError  string
}

type StatusUseCase struct {
	repo  artifactrepo.Repository
	flowc flow.TaskClient
}

func NewStatus(repo artifactrepo.Repository, flowc flow.TaskClient) *StatusUseCase {
	return &StatusUseCase{repo: repo, flowc: flowc}
}

func (u *StatusUseCase) Execute(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	a, err := u.repo.FindById(ctx, req.ArtifactId)
	if err != nil {
		return nil, err
	}
	userId := pkgcontext.GetUserId(ctx)
	if !a.IsOwner(userId) {
		return nil, artifacterrors.ErrArtifactNotOwnedByUser
	}

	if a.IsTerminal() {
		return &StatusResponse{Status: a.Status, Title: a.Title, Result: a.Result, ResultKind: a.ResultKind}, nil
	}

	if a.FlowTaskId == "" {
		return nil, artifacterrors.ErrInvalidFlowTaskId
	}

	info, err := u.flowc.Get(ctx, a.FlowTaskId)
	if err != nil {
		return nil, errors.WithMessage(err, "query flow task failed")
	}
	mapped := mapFlowState(info.State)
	return &StatusResponse{Status: mapped, FlowError: string(info.Error)}, nil
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
