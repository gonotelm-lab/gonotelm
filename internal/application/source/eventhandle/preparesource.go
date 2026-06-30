package eventhandle

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceevent "github.com/gonotelm-lab/gonotelm/internal/domain/source/event"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
)

const PreparationConsumerGroup = "gonotelm.source.preparation.group"

// PrepareSourceHandler handles source preparation events consumed from the outer bus.
type PrepareSourceHandler struct {
	sourceRepo         sourcerepo.Repository
	sourceIndexService *index.Service
	sourceStorageRepo  sourcerepo.StorageRepository
	sourceDocRepo      sourcerepo.SourceDocRepository
	summarizer         adapter.Summarizer
}

func NewPrepareSourceHandler(
	sourceRepo sourcerepo.Repository,
	sourceStorageRepo sourcerepo.StorageRepository,
	sourceDocRepo sourcerepo.SourceDocRepository,
	summarizer adapter.Summarizer,
) *PrepareSourceHandler {
	handler := &PrepareSourceHandler{
		sourceRepo:         sourceRepo,
		sourceIndexService: index.New(index.ServiceConfig{}, sourceStorageRepo, sourceDocRepo),
		sourceStorageRepo:  sourceStorageRepo,
		sourceDocRepo:      sourceDocRepo,
		summarizer:         summarizer,
	}

	return handler
}

func (h *PrepareSourceHandler) Handle(
	ctx context.Context,
	evt *sourceevent.PreparationEvent,
	env eventbus.Envelope,
) error {
	sourceId := evt.Id

	targetSource, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		return errors.WithMessagef(err, "find source failed, source_id=%s", evt.Id)
	}
	slog.DebugContext(ctx, "received and handling source preparation event",
		slog.String("source_id", sourceId.String()),
	)

	defer func() {
		if rec := recover(); rec != nil {
			slog.ErrorContext(ctx, "source preparation event handler panic",
				slog.Any("err", rec),
				slog.String("stack", string(debug.Stack())),
			)

			// 本次处理失败
			targetSource.MarkFailed()
			if err := h.sourceRepo.Save(ctx, targetSource); err != nil {
				slog.ErrorContext(ctx, "save source failed after panic",
					slog.String("source_id", sourceId.String()),
					slog.Any("err", err),
				)
			}
		}
	}()

	// 开始处理对来源进行处理 执行构建索引等操作
	if isPreparationRetry(env) {
		// clear existing indices
		if err := h.sourceDocRepo.BatchDeleteBySourceId(
			ctx,
			targetSource.NotebookId,
			[]valobj.Id{sourceId},
		); err != nil {
			slog.ErrorContext(ctx, "delete source docs failed",
				slog.String("source_id", sourceId.String()),
				slog.Any("err", err),
			)
		}

		// clear existing indices
		if err := h.sourceStorageRepo.DeleteObject(ctx, targetSource.ParsedContentKey); err != nil {
			slog.ErrorContext(ctx, "delete parsed content failed",
				slog.String("source_id", sourceId.String()),
				slog.Any("err", err),
			)
		}
	}

	result, err := h.sourceIndexService.IndexSource(ctx, targetSource)
	if err != nil {
		return errors.WithMessagef(err, "index source failed, source_id=%s", evt.Id)
	}

	if err := h.uploadParsedContent(ctx, targetSource, result); err != nil {
		return errors.WithMessagef(err, "upload parsed content failed, source_id=%s", evt.Id)
	}

	// update abstract
	if err := h.updateSourceAbstract(ctx, targetSource, result); err != nil {
		slog.ErrorContext(ctx, "update source abstract failed",
			slog.String("source_id", evt.Id.String()),
			slog.Any("err", err),
		)
	}

	if err := h.sourceRepo.Save(ctx, targetSource); err != nil {
		return errors.WithMessagef(err, "save source failed after index, source_id=%s", evt.Id)
	}

	slog.DebugContext(ctx, "source preparation completed", slog.String("source_id", evt.Id.String()))

	return nil
}

func (h *PrepareSourceHandler) uploadParsedContent(
	ctx context.Context,
	source *entity.Source,
	result *index.IndexSourceResult,
) error {
	// 上传解析完成的文档内容
	source.UploadParsedContent()
	source.MarkReady()
	if err := h.sourceStorageRepo.UploadObject(
		ctx,
		source.ParsedContentKey,
		result.ParsedContent,
		result.ParsedContentType,
	); err != nil {
		return errors.WithMessagef(err, "upload parsed content failed, source_id=%s", source.Id.String())
	}

	return nil
}

func (h *PrepareSourceHandler) updateSourceAbstract(
	ctx context.Context,
	source *entity.Source,
	result *index.IndexSourceResult,
) error {
	if len(result.SourceDocs) == 0 {
		return nil
	}

	const (
		batchSize          = 1
		maxConcurrency     = 20
		tokenSize          = 8000
		maxSummarizedChunk = 64
	)

	chunks := make([]string, 0, len(result.SourceDocs))
	for _, doc := range result.SourceDocs {
		chunks = append(chunks, doc.Content)
	}

	newChunks := pkgstring.MergeChunks(chunks, tokenSize)
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
			summary, err := h.summarizer.Summarize(ctx, batch[0])
			if err != nil {
				slog.ErrorContext(ctx, "generate summary failed",
					slog.String("source_id", source.Id.String()),
					slog.Any("err", err),
				)
				return []string{}, nil
			}

			return []string{summary}, nil
		},
	)
	if err != nil {
		return errors.WithMessagef(err, "generate summary failed, source_id=%s", source.Id.String())
	}

	// 给每个chunk的summary组合后再输出一句summary 作为整个source的summary
	summarizingTexts := strings.Join(chunkSummaries, "\n")
	summary, err := h.summarizer.Summarize(ctx, summarizingTexts)
	if err != nil {
		return errors.WithMessagef(err, "generate summary failed, source_id=%s", source.Id.String())
	}

	source.UpdateAbstract(summary)

	return nil
}

func RegisterPreparationConsumer(
	ctx context.Context,
	bus eventbus.EventBus,
	handler *PrepareSourceHandler,
) error {
	return bus.Subscribe(ctx, sourceevent.PreparationTopic, PreparationConsumerGroup,
		func(ctx context.Context, env eventbus.Envelope) error {
			var evt sourceevent.PreparationEvent
			if err := sonic.Unmarshal(env.Value, &evt); err != nil {
				return errors.Wrap(err, "unmarshal preparation event")
			}

			return handler.Handle(ctx, &evt, env)
		},
	)
}

func isPreparationRetry(env eventbus.Envelope) bool {
	val, ok := env.Header(sourceevent.PreparationRetryHeaderKey)
	return ok && string(val) == sourceevent.PreparationRetryHeaderValue
}
