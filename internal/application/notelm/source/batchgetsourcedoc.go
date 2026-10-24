package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type BatchGetSourceDocsHandler struct {
	*baseHandler
	sourceDocRepo sourcerepo.SourceDocRepository
}

func NewBatchGetSourceDocsHandler(
	sourceRepo sourcerepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	sourceDocRepo sourcerepo.SourceDocRepository,
) *BatchGetSourceDocsHandler {
	return &BatchGetSourceDocsHandler{
		baseHandler:   newBaseHandler(sourceRepo, notebookAccessService),
		sourceDocRepo: sourceDocRepo,
	}
}

type BatchGetSourceDocsHandleQuery struct {
	SourceId valobj.Id
	DocIds   []valobj.Id
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
	return withSourceAccess(h.baseHandler, ctx, cmd.SourceId, accessView,
		func(ctx context.Context, source *sourceentity.Source) (*BatchGetSourceDocsHandleResult, error) {
			return h.batchGetDocs(ctx, source, cmd)
		},
	)
}

func (h *BatchGetSourceDocsHandler) batchGetDocs(
	ctx context.Context,
	source *sourceentity.Source,
	cmd *BatchGetSourceDocsHandleQuery,
) (*BatchGetSourceDocsHandleResult, error) {
	docs, err := h.sourceDocRepo.BatchFind(ctx, source.NotebookId, source.Id, cmd.DocIds)
	if err != nil {
		return nil, errors.WithMessagef(err, "batch find source docs failed, source_id=%s", cmd.SourceId)
	}

	return &BatchGetSourceDocsHandleResult{
		SourceId:    source.Id.String(),
		SourceTitle: source.Title,
		Docs:        docs,
	}, nil
}
