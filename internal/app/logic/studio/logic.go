package studio

import (
	"context"

	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Logic struct {
	objectStorage storage.Storage

	sourceBiz   *bizsource.Biz
	notebookBiz *biznotebook.Biz
}

func NewLogic(
	objectStorage storage.Storage,
	sourceBiz *bizsource.Biz,
	notebookBiz *biznotebook.Biz,
) *Logic {
	return &Logic{
		objectStorage: objectStorage,
		sourceBiz:     sourceBiz,
		notebookBiz:   notebookBiz,
	}
}

func (l *Logic) helpGetNotebook(ctx context.Context, notebookId uuid.UUID) (*model.Notebook, error) {
	notebook, err := l.notebookBiz.GetNotebook(ctx, notebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return nil, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", notebookId)
		}
		return nil, errors.WithMessage(err, "get notebook failed")
	}

	return notebook, nil
}
