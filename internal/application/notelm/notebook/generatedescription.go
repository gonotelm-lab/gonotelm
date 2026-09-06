package notebook

import (
	"context"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

func notebookDescGenerateLockKey(userId valobj.Uid, notebookId valobj.Id) string {
	return fmt.Sprintf("notelm:notebook:desc:lock:%s:%s", userId, notebookId)
}

type GenerateDescriptionHandler struct {
	*baseHandler

	sourceRepo sourcerepo.Repository
	distLock   adapter.DistributedLock
	summarizer adapter.Summarizer
}

func NewGenerateDescriptionHandler(
	notebookRepo notebookrepo.Repository,
	sourceRepo sourcerepo.Repository,
	distLock adapter.DistributedLock,
	summarizer adapter.Summarizer,
) *GenerateDescriptionHandler {
	return &GenerateDescriptionHandler{
		baseHandler: newBaseHandler(notebookRepo),
		sourceRepo:  sourceRepo,
		distLock:    distLock,
		summarizer:  summarizer,
	}
}

func (h *GenerateDescriptionHandler) Handle(
	ctx context.Context,
	notebookId valobj.Id,
) (string, error) {
	ctx = pkgcontext.WithScene(ctx, pkgcontext.NotebookDescGenScene, notebookId.String())
	notebook, err := h.handle(ctx, notebookId)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(notebook.Description) != "" {
		return notebook.Description, nil
	}

	userId := pkgcontext.GetUserId(ctx)
	lockKey := notebookDescGenerateLockKey(userId, notebookId)
	if err := h.distLock.Lock(ctx, lockKey); err != nil {
		return "", errors.WithMessagef(err, "lock notebook description generation failed, notebook_id=%s", notebookId)
	}
	defer func() {
		_ = h.distLock.Unlock(ctx, lockKey)
	}()

	desc, err := h.generateAndSave(ctx, notebook)
	if err != nil {
		return "", err
	}

	return desc, nil
}

func (h *GenerateDescriptionHandler) generateAndSave(
	ctx context.Context,
	notebook *notebookentity.Notebook,
) (string, error) {
	sources, err := h.sourceRepo.ListByNotebookId(
		ctx, notebook.Id, &sourcerepo.ListSpec{
			Offset: 0,
			Limit:  notebookentity.MaxSourceCountAllowed,
		})
	if err != nil {
		return "", errors.WithMessagef(err, "list sources failed, notebook_id=%s", notebook.Id)
	}

	text := buildSourcesSummarizeText(sources)
	if text == "" {
		return "", notebookerrors.ErrNoSourcesForDesc
	}

	desc, err := h.summarizer.Summarize(ctx, text)
	if err != nil {
		return "", errors.WithMessagef(err, "summarize notebook description failed, notebook_id=%s", notebook.Id)
	}

	desc = pkgstring.TruncateRune(strings.TrimSpace(desc), notebookentity.MaxDescriptionLength)
	if desc == "" {
		return "", errors.WithMessagef(notebookerrors.ErrInvalidDescription, "empty summary, notebook_id=%s", notebook.Id)
	}

	if err := notebook.UpdateDescription(desc); err != nil {
		return "", errors.WithMessage(err, "update notebook description failed")
	}

	if err := h.notebookRepo.Save(ctx, notebook); err != nil {
		return "", errors.WithMessagef(err, "save notebook description failed, notebook_id=%s", notebook.Id)
	}

	return notebook.Description, nil
}

func buildSourcesSummarizeText(sources []*sourceentity.Source) string {
	var b strings.Builder
	idx := 0
	for _, source := range sources {
		if source == nil || !source.Status.IsReady() {
			continue
		}

		title := strings.TrimSpace(source.Title)
		abstract := strings.TrimSpace(source.Abstract)
		if title == "" && abstract == "" {
			continue
		}

		idx++
		fmt.Fprintf(&b, "%d. SourceTitle: %s\nSourceAbstract: %s\n", idx, title, abstract)
	}

	return strings.TrimSpace(b.String())
}
