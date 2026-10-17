package cleanup

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const notebookPageSize = 100

// NotebookCleanupRegistrar 快照 notebook 下 source/artifact 的对象 key，注册唯一一条兜底检查点。
type NotebookCleanupRegistrar struct {
	sweeper   *Sweeper
	scheduler *CheckpointScheduler
}

func NewNotebookCleanupRegistrar(sweeper *Sweeper, scheduler *CheckpointScheduler) *NotebookCleanupRegistrar {
	return &NotebookCleanupRegistrar{sweeper: sweeper, scheduler: scheduler}
}

func (r *NotebookCleanupRegistrar) Register(ctx context.Context, notebookId valobj.Id) error {
	sources, err := r.listSources(ctx, notebookId)
	if err != nil {
		return err
	}
	artifacts, err := r.listArtifacts(ctx, notebookId)
	if err != nil {
		return err
	}

	if err := r.scheduler.ScheduleNotebookCleanup(
		ctx,
		notebookId,
		collectSourceRefs(sources),
		collectArtifactRefs(artifacts),
	); err != nil {
		return errors.WithMessage(err, "register notebook cleanup checkpoint failed")
	}

	return nil
}

func (r *NotebookCleanupRegistrar) listSources(
	ctx context.Context,
	notebookId valobj.Id,
) ([]*sourceentity.Source, error) {
	sources := make([]*sourceentity.Source, 0, notebookPageSize)

	for offset := 0; ; offset += notebookPageSize {
		page, err := r.sweeper.sourceRepo.ListByNotebookId(ctx, notebookId, &sourcerepo.ListSpec{
			Limit:  notebookPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, errors.WithMessagef(err, "list sources failed, notebook_id=%s", notebookId)
		}
		if len(page) == 0 {
			break
		}

		sources = append(sources, page...)

		if len(page) < notebookPageSize {
			break
		}
	}

	return sources, nil
}

func (r *NotebookCleanupRegistrar) listArtifacts(
	ctx context.Context,
	notebookId valobj.Id,
) ([]*artifactentity.Artifact, error) {
	artifacts := make([]*artifactentity.Artifact, 0, notebookPageSize)

	for offset := 0; ; offset += notebookPageSize {
		page, err := r.sweeper.artifactRepo.ListByNotebookId(ctx, notebookId, &artifactrepo.ListSpec{
			Limit:  notebookPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, errors.WithMessagef(err, "list artifacts failed, notebook_id=%s", notebookId)
		}
		if len(page) == 0 {
			break
		}

		artifacts = append(artifacts, page...)

		if len(page) < notebookPageSize {
			break
		}
	}

	return artifacts, nil
}
