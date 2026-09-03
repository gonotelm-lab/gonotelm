package source

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourceevent "github.com/gonotelm-lab/gonotelm/internal/domain/source/event"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
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
	eventBus           eventbus.EventBus
}

func NewPrepareSourceHandler(
	sourceRepo sourcerepo.Repository,
	sourceStorageRepo sourcerepo.StorageRepository,
	sourceDocRepo sourcerepo.SourceDocRepository,
	summarizer adapter.Summarizer,
	eventBus eventbus.EventBus,
	imageInterpreter adapter.ImageInterpreter,
) *PrepareSourceHandler {
	return &PrepareSourceHandler{
		sourceRepo: sourceRepo,
		sourceIndexService: index.New(index.ServiceConfig{
			DefaultMaxSourceFileSizeBytes: entity.MaxUploadFileSizeBytes,
		}, sourceStorageRepo, sourceDocRepo, imageInterpreter),
		sourceStorageRepo: sourceStorageRepo,
		sourceDocRepo:     sourceDocRepo,
		summarizer:        summarizer,
		eventBus:          eventBus,
	}
}

func (h *PrepareSourceHandler) Handle(
	ctx context.Context,
	evt *sourceevent.PreparationEvent,
	env eventbus.Envelope,
) error {
	sourceId := evt.Id

	ctx = pkgcontext.WithScene(ctx, pkgcontext.SourcePrepareScene, sourceId.String())
	targetSource, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		if errors.Is(err, sourceerr.ErrSourceNotFound) {
			return nil
		}

		return errors.WithMessagef(err, "find source failed, source_id=%s", evt.Id)
	}

	if targetSource.Status.IsReady() {
		return nil
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

			targetSource.MarkFailed()
			if err := h.sourceRepo.Save(ctx, targetSource); err != nil {
				slog.ErrorContext(ctx, "save source failed after panic",
					slog.String("source_id", sourceId.String()),
					slog.Any("err", err),
				)
			}
		}
	}()

	if isPreparationRetry(env) {
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

		if targetSource.ParsedContentKey != "" {
			if err := h.sourceStorageRepo.DeleteObject(ctx, targetSource.ParsedContentKey); err != nil {
				slog.ErrorContext(ctx, "delete parsed content failed",
					slog.String("source_id", sourceId.String()),
					slog.Any("err", err),
				)
			}
		}
	}

	result, err := h.sourceIndexService.IndexSource(ctx, targetSource)
	if err != nil {
		slog.ErrorContext(ctx, "index source failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)

		if sourceerr.IsSourceInvalidError(err) {
			targetSource.MarkInvalid()
		} else {
			targetSource.MarkFailed()
		}

		if saveErr := h.sourceRepo.Save(ctx, targetSource); saveErr != nil {
			slog.ErrorContext(ctx, "save source failed status failed",
				slog.String("source_id", sourceId.String()),
				slog.Any("err", saveErr),
			)
		}

		// 失败不要返回err 否则会导致无法提交
		return nil
	}

	if err := h.uploadParsedContent(ctx, targetSource, result); err != nil {
		slog.ErrorContext(ctx, "upload parsed content failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)

		targetSource.MarkFailed()
		if saveErr := h.sourceRepo.Save(ctx, targetSource); saveErr != nil {
			slog.ErrorContext(ctx, "save source failed status failed",
				slog.String("source_id", sourceId.String()),
				slog.Any("err", saveErr),
			)
		}

		return nil
	}

	if err := h.updateSourceAbstract(ctx, targetSource, result); err != nil {
		slog.ErrorContext(ctx, "update source abstract failed",
			slog.String("source_id", evt.Id.String()),
			slog.Any("err", err),
		)
	}

	if err := h.sourceRepo.Save(ctx, targetSource); err != nil {
		slog.ErrorContext(ctx, "save source failed after index",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)

		return nil
	}

	for _, domainEvent := range targetSource.PullEvents() {
		if err := h.eventBus.Publish(ctx, domainEvent); err != nil {
			slog.ErrorContext(ctx, "publish source domain event failed",
				slog.String("source_id", evt.Id.String()),
				slog.String("topic", domainEvent.Topic()),
				slog.Any("err", err),
			)
		}
	}

	slog.DebugContext(ctx, "source preparation completed", slog.String("source_id", evt.Id.String()))

	return nil
}

func (h *PrepareSourceHandler) uploadParsedContent(
	ctx context.Context,
	source *entity.Source,
	result *index.IndexSourceResult,
) error {
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

	chunkSummaries, err := batch.ParallelMap(
		ctx,
		newChunks,
		batchSize,
		maxConcurrency,
		func(ctx context.Context, batch []string) ([]string, error) {
			summary, err := h.summarizer.Summarize(ctx, batch[0])
			if err != nil {
				slog.ErrorContext(ctx, "generate summary failed", slog.String("source_id", source.Id.String()), slog.Any("err", err))
				return []string{}, nil
			}

			return []string{summary}, nil
		},
	)
	if err != nil {
		return errors.WithMessagef(err, "generate summary failed, source_id=%s", source.Id.String())
	}

	summarizingTexts := strings.Join(chunkSummaries, "\n")
	summary, err := h.summarizer.Summarize(ctx, summarizingTexts)
	if err != nil {
		return errors.WithMessagef(err, "generate summary failed, source_id=%s", source.Id.String())
	}

	source.UpdateAbstract(summary)

	return nil
}

// RegisterPreparationConsumer registers the outer (MQ) consumer for source preparation.
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
