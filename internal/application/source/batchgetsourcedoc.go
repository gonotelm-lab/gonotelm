package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type BatchGetSourceDocsHandler struct {
	sourceRepo    sourcerepo.Repository
	sourceDocRepo sourcerepo.SourceDocRepository
}

func NewBatchGetSourceDocsHandler(
	sourceRepo sourcerepo.Repository,
	sourceDocRepo sourcerepo.SourceDocRepository,
) *BatchGetSourceDocsHandler {
	return &BatchGetSourceDocsHandler{
		sourceRepo:    sourceRepo,
		sourceDocRepo: sourceDocRepo,
	}
}

type BatchGetSourceDocsHandleQuery struct {
	SourceId valobj.Id
	DocIds   []string
}

type BatchGetSourceDocsHandleResult struct {
	SourceId    string
	SourceTitle string
	Docs        []*sourceentity.SourceDoc
}

func (h *BatchGetSourceDocsHandler) Handle(
	ctx context.Context,
	cmd *BatchGetSourceDocsHandleQuery,
) (*BatchGetSourceDocsHandleResult, error) {
	source, err := h.sourceRepo.FindById(ctx, cmd.SourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "find source failed, source_id=%s", cmd.SourceId)
	}

	docs, err := h.sourceDocRepo.BatchFindById(ctx, source.NotebookId, source.Id, cmd.DocIds)
	if err != nil {
		return nil, errors.WithMessagef(err, "batch find source docs failed, source_id=%s", cmd.SourceId)
	}

	return &BatchGetSourceDocsHandleResult{
		SourceId:    source.Id.String(),
		SourceTitle: source.Title,
		Docs:        docs,
	}, nil
}
