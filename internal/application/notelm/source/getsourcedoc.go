package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetSourceDocHandler struct {
	*baseHandler
	sourceDocRepo sourcerepo.SourceDocRepository
}

func NewGetSourceDocHandler(
	sourceRepo sourcerepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	sourceDocRepo sourcerepo.SourceDocRepository,
) *GetSourceDocHandler {
	return &GetSourceDocHandler{
		baseHandler:   newBaseHandler(sourceRepo, notebookAccessService),
		sourceDocRepo: sourceDocRepo,
	}
}

type GetSourceDocHandleQuery struct {
	SourceId valobj.Id
	DocId    valobj.Id
}

type GetSourceDocHandleResult struct {
	SourceId    string
	SourceTitle string
	Doc         *sourceentity.SourceDoc
}

func (h *GetSourceDocHandler) Handle(
	ctx context.Context,
	cmd *GetSourceDocHandleQuery,
) (*GetSourceDocHandleResult, error) {
	source, err := h.handleView(ctx, cmd.SourceId)
	if err != nil {
		return nil, err
	}

	doc, err := h.sourceDocRepo.FindById(ctx, source.NotebookId, source.Id, cmd.DocId)
	if err != nil {
		return nil, errors.WithMessagef(err, "find source doc failed, source_id=%s, doc_id=%s", cmd.SourceId, cmd.DocId)
	}

	return &GetSourceDocHandleResult{
		SourceId:    source.Id.String(),
		SourceTitle: source.Title,
		Doc:         doc,
	}, nil
}
