package usecase

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListRequest struct {
	NotebookId valobj.Id
	Limit      int
	Offset     int
}

type ListResponse struct {
	Artifacts []*artifactentity.Artifact
	HasMore   bool
}

type ListUseCase struct {
	repo     artifactrepo.Repository
	notebook notebookrepo.Repository
}

func NewList(repo artifactrepo.Repository, notebook notebookrepo.Repository) *ListUseCase {
	return &ListUseCase{repo: repo, notebook: notebook}
}

func (u *ListUseCase) Execute(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	userId := pkgcontext.GetUserId(ctx)
	nb, err := u.notebook.FindById(ctx, req.NotebookId)
	if err != nil {
		return nil, err
	}
	if nb.OwnerId != userId {
		return nil, errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", req.NotebookId)
	}

	fetchLimit := req.Limit + 1
	rows, err := u.repo.ListByNotebookId(ctx, req.NotebookId, fetchLimit, req.Offset)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > req.Limit
	if hasMore {
		rows = rows[:req.Limit]
	}
	return &ListResponse{Artifacts: rows, HasMore: hasMore}, nil
}
