package source

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourceevent "github.com/gonotelm-lab/gonotelm/internal/domain/source/event"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index/convertdoc"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
)

const PreparationConsumerGroup = "gonotelm.source.preparation.group"

// PrepareSourceHandler handles source preparation events consumed from the inter-process bus (MQ).
type PrepareSourceHandler struct {
	sourceRepo         sourcerepo.Repository
	notebookRepo       notebookrepo.Repository
	sourceIndexService *index.Service
	objectStore        adapter.ObjectStore
	keyFactory         adapter.StoreKeyFactory
	sourceDocRepo      sourcerepo.SourceDocRepository
	summarizer         adapter.Summarizer
	eventBus           eventbus.Publisher
}

func NewPrepareSourceHandler(
	sourceRepo sourcerepo.Repository,
	notebookRepo notebookrepo.Repository,
	objectStore adapter.ObjectStore,
	keyFactory adapter.StoreKeyFactory,
	sourceDocRepo sourcerepo.SourceDocRepository,
	summarizer adapter.Summarizer,
	eventBus eventbus.Publisher,
	imageInterpreter adapter.ImageInterpreter,
) *PrepareSourceHandler {
	return &PrepareSourceHandler{
		sourceRepo:   sourceRepo,
		notebookRepo: notebookRepo,
		sourceIndexService: index.New(index.ServiceConfig{
			DefaultMaxSourceFileSizeBytes: entity.MaxUploadFileSizeBytes,
		}, objectStore, sourceDocRepo, imageInterpreter),
		objectStore:   objectStore,
		keyFactory:    keyFactory,
		sourceDocRepo: sourceDocRepo,
		summarizer:    summarizer,
		eventBus:      eventBus,
	}
}

func (h *PrepareSourceHandler) Handle(
	ctx context.Context,
	evt *sourceevent.PreparationEvent,
	env eventbus.Envelope,
) (err error) {
	sourceId := evt.Id

	ctx = pkgcontext.WithScene(ctx, pkgcontext.SourcePrepareScene, sourceId.String())
	ctx = pkgcontext.WithNotebookId(ctx, evt.NotebookId)

	var targetSource *entity.Source

	defer func() {
		if rec := recover(); rec != nil {
			slog.ErrorContext(ctx, "source preparation panic",
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)

			if targetSource != nil {
				targetSource.MarkFailed()
				h.persistUnlessObsolete(ctx, targetSource)
			}

			// 统一吞掉错误 不要阻塞消息提交
			err = nil

			return
		}

		if err == nil {
			return
		}

		slog.ErrorContext(ctx, "source preparation failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)

		// 统一吞掉错误 不要阻塞消息提交
		err = nil
	}()

	if !h.notebookExists(ctx, evt.NotebookId) {
		slog.InfoContext(ctx, "notebook already deleted, skip source preparation",
			slog.String("source_id", sourceId.String()),
			slog.String("notebook_id", evt.NotebookId.String()),
		)

		return nil
	}

	targetSource, err = h.sourceRepo.FindById(ctx, sourceId)
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

		if targetSource.ParsedContentKey.Valid() {
			if err := h.objectStore.DeleteObject(ctx, targetSource.ParsedContentKey); err != nil {
				slog.ErrorContext(ctx, "delete parsed content failed",
					slog.String("source_id", sourceId.String()),
					slog.Any("err", err),
				)
			}
		}
	}

	result, err := h.sourceIndexService.IndexSource(ctx, targetSource)
	if err != nil {
		if sourceerr.IsSourceInvalidError(err) {
			targetSource.MarkInvalid()
		} else {
			targetSource.MarkFailed()
		}

		// IndexSource 按 partition 并发写入，可能只写进去一部分，失败时清掉。
		h.deleteIndexDocs(ctx, targetSource)
		h.persistUnlessObsolete(ctx, targetSource)

		return errors.WithMessage(err, "index source failed")
	}

	if err := h.uploadParsedContent(ctx, targetSource, result); err != nil {
		targetSource.MarkFailed()
		h.persistUnlessObsolete(ctx, targetSource)

		return errors.WithMessage(err, "upload parsed content failed")
	}

	newTitle := ""
	if targetSource.Kind.IsUrl() {
		newTitle = convertdoc.ExtractUrlWebTitle(result.Extras)
	}
	if err := h.updateSourceAbstractTitle(ctx, targetSource, result, newTitle); err != nil {
		slog.ErrorContext(ctx, "update source abstract failed",
			slog.String("source_id", evt.Id.String()),
			slog.Any("err", err),
		)
	}

	if !h.persistUnlessObsolete(ctx, targetSource) {
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
	if err := source.UploadParsedContent(h.keyFactory); err != nil {
		return errors.WithMessagef(err, "create parsed content store key failed, source_id=%s", source.Id.String())
	}
	source.MarkReady()
	if err := h.objectStore.Upload(
		ctx,
		source.ParsedContentKey,
		result.ParsedContent,
		result.ParsedContentType,
	); err != nil {
		return errors.WithMessagef(err, "upload parsed content failed, source_id=%s", source.Id.String())
	}

	return nil
}

// notebookExists 检查 notebook 是否还在；查询失败或事件未带 notebook_id 时按"还在"处理，避免误跳过解析。
func (h *PrepareSourceHandler) notebookExists(ctx context.Context, notebookId valobj.Id) bool {
	if notebookId.IsZero() {
		return true
	}

	exists, err := h.notebookRepo.Exists(ctx, notebookId)
	if err != nil {
		slog.ErrorContext(ctx, "check notebook exists failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)

		return true
	}

	return exists
}

// persistUnlessObsolete 落库前的最后检查：notebook 或 source 行已不存在时放弃写入并尽力回滚。
func (h *PrepareSourceHandler) persistUnlessObsolete(ctx context.Context, source *entity.Source) bool {
	if !h.notebookExists(ctx, source.NotebookId) {
		slog.WarnContext(ctx, "notebook already deleted, rollback prepared source",
			slog.String("source_id", source.Id.String()),
			slog.String("notebook_id", source.NotebookId.String()),
		)
		h.rollback(ctx, source)

		return false
	}

	affected, err := h.sourceRepo.UpdateExisting(ctx, source)
	if err != nil {
		slog.ErrorContext(ctx, "save source failed",
			slog.String("source_id", source.Id.String()),
			slog.Any("err", err),
		)

		return false
	}

	if affected == 0 {
		slog.WarnContext(ctx, "source row already deleted, rollback prepared source",
			slog.String("source_id", source.Id.String()),
		)
		h.rollback(ctx, source)

		return false
	}

	return true
}

// deleteIndexDocs 删除该 source 已写入的索引文档，失败只记日志。
func (h *PrepareSourceHandler) deleteIndexDocs(ctx context.Context, source *entity.Source) {
	if err := h.sourceDocRepo.BatchDeleteBySourceId(
		ctx,
		source.NotebookId,
		[]valobj.Id{source.Id},
	); err != nil {
		slog.ErrorContext(ctx, "delete source docs failed",
			slog.String("source_id", source.Id.String()),
			slog.Any("err", err),
		)
	}
}

// rollback 尽力回滚本次解析的副作用（索引文档 + parsed 对象），失败只记日志；
// durable 的重试由 notelm 在删除时注册的一次性回调检查点负责。
func (h *PrepareSourceHandler) rollback(ctx context.Context, source *entity.Source) {
	h.deleteIndexDocs(ctx, source)

	if source.ParsedContentKey.Valid() {
		if err := h.objectStore.DeleteObject(ctx, source.ParsedContentKey); err != nil {
			slog.ErrorContext(ctx, "rollback parsed content object failed",
				slog.String("source_id", source.Id.String()),
				slog.String("store_key", source.ParsedContentKey.String()),
				slog.Any("err", err),
			)
		}
	}
}

func (h *PrepareSourceHandler) updateSourceAbstractTitle(
	ctx context.Context,
	source *entity.Source,
	result *index.IndexSourceResult,
	newTitle string,
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
	if newTitle != "" {
		source.UpdateTitle(newTitle)
	}

	return nil
}

// RegisterPreparationConsumer registers the inter-process (MQ) consumer for source preparation.
func RegisterPreparationConsumer(
	ctx context.Context,
	bus eventbus.InterProcessEventBus,
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
