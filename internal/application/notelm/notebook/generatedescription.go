package notebook

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

func notebookDescGenerateLockKey(notebookId valobj.Id) string {
	return fmt.Sprintf("notelm:notebook:desc:lock:%s", notebookId)
}

type GenerateDescriptionHandler struct {
	*baseHandler

	sourceRepo sourcerepo.Repository
	distLock   adapter.DistributedLock
	summarizer adapter.Summarizer
	aiDescRepo notebookrepo.AIDescriptionRepository
}

func NewGenerateDescriptionHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	sourceRepo sourcerepo.Repository,
	userRepo identrepo.UserRepository,
	distLock adapter.DistributedLock,
	summarizer adapter.Summarizer,
	aiDescRepo notebookrepo.AIDescriptionRepository,
) *GenerateDescriptionHandler {
	return &GenerateDescriptionHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo, nil),
		sourceRepo:  sourceRepo,
		distLock:    distLock,
		summarizer:  summarizer,
		aiDescRepo:  aiDescRepo,
	}
}

func (h *GenerateDescriptionHandler) Handle(
	ctx context.Context,
	notebookId valobj.Id,
) (string, error) {
	ctx = pkgcontext.WithScene(ctx, pkgcontext.NotebookDescGenScene, notebookId.String())
	return withNotebookAccess(h.baseHandler, ctx, notebookId, accessView, h.generateDescription)
}

func (h *GenerateDescriptionHandler) generateDescription(
	ctx context.Context,
	access *notebookentity.Access,
) (string, error) {
	notebook := access.Notebook

	// 已设自定义摘要时禁止 AI 生成，避免覆盖用户自定义的摘要
	if notebook.CustomizeDescription() {
		return "", notebookerrors.ErrCustomizeDescription
	}

	sources, err := h.sourceRepo.ListByNotebookId(
		ctx, notebook.Id, &sourcerepo.ListSpec{
			Offset: 0,
			Limit:  notebookentity.MaxSourceCountAllowed,
		})
	if err != nil {
		return "", errors.WithMessagef(err, "list sources failed, notebook_id=%s", notebook.Id)
	}

	// 仅用 ready 来源的 title/abstract 拼 prompt；无可摘要内容返回空串（不报错）
	text := h.buildSourcesSummarizeText(sources)
	if text == "" {
		return "", nil
	}

	// 指纹随来源集合变化；同指纹命中缓存则跳过 LLM
	fp := sourceentity.NewFingerprint(sources)
	if cached, err := h.getCachedDescription(ctx, notebook.Id, fp); err != nil {
		return "", err
	} else if cached != "" {
		return cached, nil
	}

	// 持锁生成，避免并发打多次 LLM；锁内再读一次缓存（double-check）
	var desc string
	err = h.distLock.WithLockWithRetry(
		ctx,
		notebookDescGenerateLockKey(notebook.Id),
		adapter.DefaultRetrySpec,
		func(ctx context.Context) error {
			if cached, err := h.getCachedDescription(ctx, notebook.Id, fp); err != nil {
				return err
			} else if cached != "" {
				desc = cached
				return nil
			}

			generated, err := h.summarizeAndSave(ctx, notebook.Id, fp, text)
			if err != nil {
				return err
			}
			desc = generated
			return nil
		})
	if err == nil {
		return desc, nil
	}
	if !errors.Is(err, errors.ErrLockNotAcquired) {
		return "", err
	}

	// 重试后仍抢不到锁：无锁独立生成并落缓存，宁可偶发重复调用 LLM
	slog.WarnContext(ctx, "notebook desc lock contended, generate independently",
		slog.String("notebook_id", notebook.Id.String()),
		slog.Any("err", err),
	)
	if cached, err := h.getCachedDescription(ctx, notebook.Id, fp); err != nil {
		return "", err
	} else if cached != "" {
		return cached, nil
	}
	return h.summarizeAndSave(ctx, notebook.Id, fp, text)
}

func (h *GenerateDescriptionHandler) summarizeAndSave(
	ctx context.Context,
	notebookId valobj.Id,
	fp string,
	text string,
) (string, error) {
	generated, err := h.summarizer.Summarize(ctx, text)
	if err != nil {
		return "", errors.WithMessagef(err, "summarize notebook description failed, notebook_id=%s", notebookId)
	}

	generated = pkgstring.TruncateRune(strings.TrimSpace(generated), notebookentity.MaxDescriptionLength)
	if generated == "" {
		return "", errors.WithMessagef(notebookerrors.ErrInvalidDescription, "empty summary, notebook_id=%s", notebookId)
	}

	if err := h.aiDescRepo.Save(ctx, notebookId, fp, generated); err != nil {
		return "", errors.WithMessagef(err, "save ai description failed, notebook_id=%s", notebookId)
	}
	return generated, nil
}

func (h *GenerateDescriptionHandler) getCachedDescription(
	ctx context.Context,
	notebookId valobj.Id,
	fp string,
) (string, error) {
	cached, err := h.aiDescRepo.Get(ctx, notebookId, fp)
	if err != nil {
		return "", errors.WithMessagef(err, "get ai description failed, notebook_id=%s", notebookId)
	}
	return cached, nil
}

func (h *GenerateDescriptionHandler) buildSourcesSummarizeText(sources []*sourceentity.Source) string {
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
