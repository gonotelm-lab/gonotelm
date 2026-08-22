package source

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

type CreateSourceHandler struct {
	notebookRepo notebookrepo.Repository
	sourceRepo   sourcerepo.Repository
	eventBus     eventbus.EventBus
}

func NewCreateSourceHandler(
	sourceRepo sourcerepo.Repository,
	notebookRepo notebookrepo.Repository,
	eventBus eventbus.EventBus,
) *CreateSourceHandler {
	return &CreateSourceHandler{
		notebookRepo: notebookRepo,
		sourceRepo:   sourceRepo,
		eventBus:     eventBus,
	}
}

type CreateSourceHandleCommand struct {
	NotebookId valobj.Id
	OwnerId    valobj.Uid
	Kind       sourcevo.SourceKind
	Text       string
	Url        *url.URL
}

func (h *CreateSourceHandler) Handle(
	ctx context.Context,
	cmd *CreateSourceHandleCommand,
) (valobj.Id, error) {
	var newId valobj.Id
	if cmd.Kind.IsUrl() && cmd.Url != nil {
		if err := checkURLBlacklist(cmd.Url); err != nil {
			return newId, err
		}
	}

	userId := pkgcontext.GetUserId(ctx)
	curNotebook, err := h.notebookRepo.FindById(ctx, cmd.NotebookId)
	if err != nil {
		return newId, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", cmd.NotebookId)
	}
	if curNotebook.OwnerId != userId {
		return newId, errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", cmd.NotebookId)
	}

	if err := curNotebook.AllowedToCreateSource(); err != nil {
		return newId, errors.WithMessage(err, "notebook not allowed to create source")
	}

	newSource, err := sourceentity.NewSource(
		cmd.NotebookId,
		cmd.Kind,
		cmd.OwnerId,
		&sourceentity.ContentUnion{
			Kind: cmd.Kind,
			Text: cmd.Text,
			Url:  cmd.Url,
		},
	)
	if err != nil {
		return newId, errors.WithMessage(err, "create source failed")
	}

	var sourceTitle string
	if newSource.Kind.IsText() {
		sourceTitle = pkgstring.TruncateRune(cmd.Text, 64)
	} else if newSource.Kind.IsUrl() {
		sourceTitle = pkgstring.TruncateRune(cmd.Url.String(), sourceentity.MaxSourceTitleLength)
	}
	newSource.UpdateTitle(sourceTitle)

	err = h.sourceRepo.Save(ctx, newSource)
	if err != nil {
		return newId, errors.WithMessage(err, "save source failed")
	}

	// send source created event
	events := newSource.PullEvents()
	slog.DebugContext(ctx, "source created",
		slog.String("source_id", newSource.Id.String()),
		slog.Int("num_events", len(events)),
	)
	for _, event := range events {
		err = h.eventBus.Publish(ctx, event)
		if err != nil {
			slog.ErrorContext(ctx, "publish source created event failed", "error", err, "source_id", newSource.Id)
		}
	}

	return newSource.Id, nil
}

func checkURLBlacklist(u *url.URL) error {
	blacklistRegex := conf.NotelmGlobal().Source.GetURLBlacklistRegex()
	if blacklistRegex.MatchString(u.String()) {
		return errors.ErrParams.Msgf("url not allowed: %s", u.String())
	}

	return nil
}
