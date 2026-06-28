package source

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CreateSourceHandler struct {
	sourceRepo   source.Repository
	notebookRepo notebook.Repository
	eventBus     eventbus.EventBus
}

func NewCreateSourceHandler(
	sourceRepo source.Repository,
	notebookRepo notebook.Repository,
	eventBus eventbus.EventBus,
) *CreateSourceHandler {
	return &CreateSourceHandler{
		sourceRepo:   sourceRepo,
		notebookRepo: notebookRepo,
		eventBus:     eventBus,
	}
}

type CreateSourceHandleCommand struct {
	NotebookId valobj.Id
	OwnerId    string
	Kind       source.SourceKind
	Text       string
	Url        *url.URL
}

func (h *CreateSourceHandler) Handle(
	ctx context.Context,
	cmd *CreateSourceHandleCommand,
) (valobj.Id, error) {
	var newId valobj.Id
	curNotebook, err := h.notebookRepo.FindById(ctx, cmd.NotebookId)
	if err != nil {
		return newId, errors.WithMessage(err, "get notebook failed")
	}

	if err := curNotebook.AllowedToCreateSource(); err != nil {
		return newId, errors.WithMessage(err, "notebook not allowed to create source")
	}

	newSource, err := source.NewSource(
		cmd.NotebookId,
		cmd.Kind,
		cmd.OwnerId,
		&source.ContentIntegrate{
			Kind: cmd.Kind,
			Text: cmd.Text,
			Url:  cmd.Url,
		},
	)
	if err != nil {
		return newId, errors.WithMessage(err, "create source failed")
	}

	err = h.sourceRepo.Save(ctx, newSource)
	if err != nil {
		return newId, errors.WithMessage(err, "save source failed")
	}

	// send source created event
	events := newSource.PullEvents()
	for _, event := range events {
		err = h.eventBus.Publish(ctx, event)
		if err != nil {
			slog.ErrorContext(ctx, "publish source created event failed", "error", err, "source_id", newSource.Id)
		}
	}

	return newSource.Id, nil
}
