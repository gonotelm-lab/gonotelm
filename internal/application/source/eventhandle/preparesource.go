package eventhandle

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/bytedance/sonic"
	domain "github.com/gonotelm-lab/gonotelm/internal/domain/source"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const PreparationConsumerGroup = "gonotelm.source.preparation.group"

// PrepareSourceHandler handles source preparation events consumed from the outer bus.
type PrepareSourceHandler struct {
	sourceRepo domain.Repository
}

func NewPrepareSourceHandler(sourceRepo domain.Repository) *PrepareSourceHandler {
	return &PrepareSourceHandler{
		sourceRepo: sourceRepo,
	}
}

func (h *PrepareSourceHandler) Handle(
	ctx context.Context,
	evt *domain.PreparationEvent,
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
		// TODO clear existing indices
	}



	// TODO: migrate preparation workflow from internal/app/logic/source/eventhandle.go
	return nil
}

func RegisterPreparationConsumer(
	ctx context.Context,
	bus eventbus.EventBus,
	handler *PrepareSourceHandler,
) error {
	return bus.Subscribe(ctx, domain.PreparationTopic, PreparationConsumerGroup,
		func(ctx context.Context, env eventbus.Envelope) error {
			var evt domain.PreparationEvent
			if err := sonic.Unmarshal(env.Value, &evt); err != nil {
				return errors.Wrap(err, "unmarshal preparation event")
			}

			return handler.Handle(ctx, &evt, env)
		},
	)
}

func isPreparationRetry(env eventbus.Envelope) bool {
	val, ok := env.Header(domain.PreparationRetryHeaderKey)
	return ok && string(val) == domain.PreparationRetryHeaderValue
}
