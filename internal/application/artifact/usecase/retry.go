package usecase

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type RetryUseCase struct {
	repo   artifactrepo.Repository
	flowc  flow.TaskClient
	poller Poller
}

func NewRetry(repo artifactrepo.Repository, flowc flow.TaskClient, poller Poller) *RetryUseCase {
	return &RetryUseCase{repo: repo, flowc: flowc, poller: poller}
}

func (u *RetryUseCase) Execute(ctx context.Context, artifactId valobj.Id) error {
	a, err := u.repo.FindById(ctx, artifactId)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	if a.Status != artifactentity.StatusFailed && a.Status != artifactentity.StatusCancelled {
		return artifacterrors.ErrCannotRetryInState
	}

	oldFlowTaskId := a.FlowTaskId
	payloadBytes, err := sonic.Marshal(a.Payload)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal payload on retry err=%v", err)
	}
	newFlowTaskId, err := u.flowc.Submit(ctx, taskTypeFor(a.Kind), payloadBytes)
	if err != nil {
		return errors.WithMessage(err, "submit retry task to flow failed")
	}

	if err := u.repo.UpdateFlowTaskId(ctx, a.Id, newFlowTaskId, []artifactentity.Status{artifactentity.StatusFailed, artifactentity.StatusCancelled}); err != nil {
		return errors.WithMessage(err, "update flow task id failed")
	}
	if err := u.repo.UpdateStatus(ctx, a.Id, artifactentity.StatusPending, nil, "", ""); err != nil {
		return errors.WithMessage(err, "save retried artifact failed")
	}

	if oldFlowTaskId != "" && oldFlowTaskId != newFlowTaskId {
		go func() { _ = u.flowc.Cancel(context.WithoutCancel(ctx), oldFlowTaskId) }()
	}
	if u.poller != nil {
		go u.poller.PollOne(context.WithoutCancel(ctx), a.Id)
	}
	return nil
}
