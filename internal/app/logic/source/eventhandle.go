package source

import (
	"context"
	"log/slog"
	"strings"

	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
)

const (
	sourcePrepareRetryKey   = "x-source-prepare-retry"
	sourcePrepareRetryValue = "true"
)

func (l *Logic) generateNotebookSummary(
	ctx context.Context,
	notebookId uuid.UUID,
) {
	slog.DebugContext(ctx, "generate notebook summary",
		slog.String("notebook_id", notebookId.String()),
	)

	notebook, err := l.notebookBiz.GetNotebook(ctx, notebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return
		}

		slog.ErrorContext(ctx, "get notebook failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	if notebook.Description != "" {
		// 自动生成的描述不覆盖已有的
		return
	}

	// get all notebook sources
	notebookSources, err := l.sourceBiz.FetchNotebookSources(ctx, notebookId)
	if err != nil {
		slog.ErrorContext(ctx, "get all notebook sources failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	abstracts := make([]string, 0, len(notebookSources))
	for _, source := range notebookSources {
		if source.Abstract != "" {
			abstracts = append(abstracts, source.Abstract)
		}
	}

	// generate prompt message
	msgs, err := l.prompt.RenderNotebookSummaryMessage(
		ctx, abstracts, pkgcontext.GetLang(ctx),
	)
	if err != nil {
		slog.ErrorContext(ctx, "render notebook summary prompt failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	var (
		provider = conf.Global().Logic.Source.ModelProvider
		model    = conf.Global().Logic.Source.Model
	)

	// generate summary
	chatModel, err := l.llmGateway.GetProvider(provider)
	if err != nil {
		slog.ErrorContext(ctx, "get summary model failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}
	result, err := chatModel.Generate(
		ctx,
		msgs,
		llmchat.WithModel(model),
		llmchat.WithResponseJsonObject(provider),
	)
	if err != nil {
		slog.ErrorContext(ctx, "generate notebook summary failed",
			slog.String("notebook_id", notebookId.String()), slog.Any("err", err),
		)
		return
	}

	// now we update notebook description
	expect := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Valid       bool   `json:"valid"`
	}{}

	// truncate
	expect.Name = strings.TrimSpace(expect.Name)
	expect.Description = strings.TrimSpace(expect.Description)
	expect.Name = constants.TruncateNotebookName(expect.Name)
	expect.Description = constants.TruncateNotebookDescription(expect.Description)

	err = sonic.Unmarshal(pkgstring.AsBytes(result.Content), &expect)
	if err != nil {
		slog.WarnContext(ctx, "llm model response unmarshal failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	if !expect.Valid {
		slog.WarnContext(ctx, "notebook summary is not valid",
			slog.String("notebook_id", notebookId.String()),
		)
		return
	}

	slog.DebugContext(ctx, "update notebook description",
		slog.String("notebook_id", notebookId.String()),
	)

	err = l.notebookBiz.FillNotebookMeta(ctx,
		&biznotebook.FillNotebookMetaCommand{
			Id:          notebookId,
			Name:        expect.Name,
			Description: expect.Description,
		})
	if err != nil {
		slog.ErrorContext(ctx, "fill notebook meta failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}
}
