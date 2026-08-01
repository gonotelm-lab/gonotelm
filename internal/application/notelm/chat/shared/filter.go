package shared

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func FilterReadySources(
	ctx context.Context,
	sourceRepo sourcerepo.Repository,
	notebookId valobj.Id,
	sourceIds []valobj.Id,
	userId string,
) ([]*sourceentity.Source, error) {
	if len(sourceIds) == 0 {
		return nil, nil
	}

	sources, err := sourceRepo.GetByNotebookIdAndIds(ctx, notebookId, sourceIds)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to get sources, notebook_id=%s, source_ids=%v", notebookId, sourceIds)
	}

	// filter status
	readySources := make([]*sourceentity.Source, 0, len(sources))
	for _, source := range sources {
		if source.Status.IsReady() && source.OwnerId == userId {
			readySources = append(readySources, source)
		}
	}

	if len(sources) == 0 {
		return nil, errors.ErrParams.Msgf("no ready sources found, notebook_id=%s, source_ids=%v", notebookId, sourceIds)
	}

	return readySources, nil
}
