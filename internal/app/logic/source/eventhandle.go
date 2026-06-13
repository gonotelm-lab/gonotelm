package source

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"

	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/mq"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	sourcePrepareRetryKey   = "x-source-prepare-retry"
	sourcePrepareRetryValue = "true"
)

// logic event handler
func (l *Logic) notifySourceEventMessage(
	ctx context.Context,
	source *model.Source,
	isRetry bool,
) error {
	sourceEvent := &sourceEventMessage{
		Id:         source.Id,
		NotebookId: source.NotebookId,
		Kind:       source.Kind,
		Status:     source.Status,
		UserId:     source.OwnerId,
	}
	value, err := sonic.Marshal(sourceEvent)
	if err != nil {
		return errors.Wrapf(err,
			"marshal source failed, kind=%s, source_id=%s",
			source.Kind, source.Id)
	}

	var header []mq.MessageHeader
	if isRetry {
		header = append(header, mq.MessageHeader{
			Key:   sourcePrepareRetryKey,
			Value: []byte(sourcePrepareRetryValue),
		})
	}

	err = l.prepProducer.Send(ctx, &mq.ProducerSendRequest{
		Topic:   TopicSourcePreparation,
		Key:     []byte(source.Id.String()),
		Value:   value,
		Headers: header,
	})
	if err != nil {
		return errors.Wrapf(err,
			"send source inserted message failed, kind=%s, source_id=%s",
			source.Kind, source.Id)
	}

	return nil
}

// 消息队列消费 消费上传完成的任务 指定来源索引构建
func (l *Logic) handleSourceEventMessage(
	ctx context.Context,
	msg mq.Message,
) error {
	var (
		key = msg.Key()
		val = msg.Value()

		sourceEvent sourceEventMessage
		err         error
	)

	sourceId, err := uuid.FromBytes(key)
	if err != nil {
		return errors.Wrapf(err, "parse source id failed, key=%s", string(key))
	}

	ctx = pkgcontext.WithUserId(ctx, sourceEvent.UserId)
	slog.DebugContext(ctx, "received and handling source prep message", slog.String("msg_key", sourceId.String()))
	err = sonic.Unmarshal(val, &sourceEvent)
	if err != nil {
		return errors.Wrap(err, "handle prep message unmarshal failed")
	}

	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "source prep message handler panic",
				slog.Any("err", r),
				slog.String("stack", string(debug.Stack())),
			)

			// 本次处理失败
			if uerr := l.sourceBiz.UpdateStatus(
				ctx,
				sourceEvent.Id,
				model.SourceStatusFailed,
			); uerr != nil {
				slog.ErrorContext(ctx, "update source status failed after recover from a panicking",
					slog.String("source_id", sourceEvent.Id.String()),
					slog.Any("err", uerr),
				)
			}
		}
	}()

	source, err := l.sourceBiz.GetDecodedSource(ctx, sourceEvent.Id)
	if err != nil {
		if errors.Is(err, bizsource.ErrSourceNotFound) {
			return nil
		}

		return errors.WithMessagef(err, "get source failed, id=%s", sourceEvent.Id)
	}

	isRetry := false
	for _, h := range msg.Headers() {
		if h.Key == sourcePrepareRetryKey {
			if h.Value != nil && pkgstring.FromBytes(h.Value) == sourcePrepareRetryValue {
				isRetry = true
			}
			break
		}
	}

	if isRetry {
		// clear original
		if err := l.sourceBiz.ClearSourceIndices(
			ctx,
			sourceEvent.NotebookId,
			sourceEvent.Id,
		); err != nil {
			slog.ErrorContext(ctx, "clear source indices failed",
				slog.String("source_id", sourceEvent.Id.String()),
				slog.Any("err", err),
			)
		}

		// delete parsed content if necessary
		if err := l.sourceBiz.DeleteParsedContent(ctx, source); err != nil {
			slog.ErrorContext(ctx, "delete parsed content failed",
				slog.String("source_id", sourceEvent.Id.String()),
				slog.Any("err", err),
			)
		}
	}

	result, err := l.sourceBiz.PrepareSourceIndices(ctx, sourceEvent.Id)
	if err != nil {
		// mark failure
		err2 := l.sourceBiz.UpdateStatus(ctx, sourceEvent.Id, model.SourceStatusFailed)
		if err2 != nil {
			return errors.WithMessage(err2, "update source status failed")
		}

		if errors.Is(err, bizsource.ErrSourceContentTooLong) {
			slog.WarnContext(ctx, "source content too long",
				slog.String("source_id", sourceEvent.Id.String()),
				slog.Any("err", err),
			)
			return nil
		}

		slog.ErrorContext(ctx, "prepare source failed",
			slog.String("source_id", sourceEvent.Id.String()),
			slog.Any("err", err),
		)
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(2)
	l.wg.Go(func() {
		defer wg.Done()
		if err := l.sourceBiz.UploadParsedContent(ctx,
			&bizsource.UploadParsedContentCommand{
				SourceId:          sourceEvent.Id,
				NotebookId:        sourceEvent.NotebookId,
				ParsedContent:     result.ParsedContent,
				ParsedContentType: result.ParsedContentType,
			}); err != nil {
			// 解析成功但是上传失败仅记录日志，不影响后续流程
			slog.ErrorContext(ctx, "upload parsed content failed",
				slog.String("source_id", sourceEvent.Id.String()),
				slog.Any("err", err),
			)
		}
	})
	l.wg.Go(func() {
		defer wg.Done()
		l.generateSourceSummary(ctx, sourceEvent.Id, sourceEvent.NotebookId, result)
	})
	wg.Wait()

	l.generateNotebookSummary(ctx, sourceEvent.NotebookId)

	// ok
	err = l.sourceBiz.UpdateStatus(ctx, sourceEvent.Id, model.SourceStatusReady)
	if err != nil {
		return errors.WithMessage(err, "update source status failed")
	}

	slog.DebugContext(ctx, "prepared source success", "source_id", sourceEvent.Id)

	return nil
}

func (l *Logic) generateSourceSummary(
	ctx context.Context,
	sourceId uuid.UUID,
	notebookId uuid.UUID,
	result *bizsource.PrepareSourceIndicesResult,
) {
	if len(result.Chunks) == 0 {
		return
	}

	const (
		batchSize          = 1
		maxConcurrency     = 20
		tokenSize          = 8000
		maxSummarizedChunk = 64
	)

	newChunks := pkgstring.MergeChunks(result.Chunks, tokenSize)
	if len(newChunks) > maxSummarizedChunk {
		newChunks = newChunks[:maxSummarizedChunk]
	}

	// newChunks中每个元素都生成一份摘要
	chunkSummaries, err := batch.ParallelMap(
		ctx,
		newChunks,
		batchSize,
		maxConcurrency,
		func(ctx context.Context, batch []string) ([]string, error) {
			summary, err := l.summarizer.Summarize(ctx, batch[0])
			if err != nil {
				slog.ErrorContext(ctx, "generate summary failed",
					slog.String("source_id", sourceId.String()),
					slog.Any("err", err),
				)
				return []string{}, nil
			}

			return []string{summary}, nil
		},
	)
	if err != nil {
		slog.ErrorContext(ctx, "generate summary failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)
		return
	}

	// 给每个chunk的summary组合后再输出一句summary 作为整个source的summary
	summarizingTexts := strings.Join(chunkSummaries, "\n")
	summary, err := l.summarizer.Summarize(ctx, summarizingTexts)
	if err != nil {
		return
	}
	if err := l.sourceBiz.UpdateAbstract(ctx, sourceId, summary); err != nil {
		slog.ErrorContext(ctx, "update source abstract failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)
	}
}

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
	msg, err := prompts.RenderNotebookSummaryMessage(
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
		[]*einoschema.Message{msg},
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
