package repository

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type StreamTaskRepositoryImpl struct {
	streamCache cache.ChatMessageStreamCache
}

func NewStreamTaskRepository(streamCache cache.ChatMessageStreamCache) chatrepo.StreamTaskRepository {
	return &StreamTaskRepositoryImpl{
		streamCache: streamCache,
	}
}

var _ chatrepo.StreamTaskRepository = &StreamTaskRepositoryImpl{}

func (r *StreamTaskRepositoryImpl) Save(ctx context.Context, task *entity.StreamTask) error {
	sch := mapper.StreamTaskToSchema(task)
	_, err := r.streamCache.SetTask(ctx, sch)
	return err
}

func (r *StreamTaskRepositoryImpl) FindById(ctx context.Context, taskId valobj.Id) (*entity.StreamTask, error) {
	sch, err := r.streamCache.GetTask(ctx, taskId.String())
	if err != nil {
		if errors.Is(err, cache.ErrTaskNotFound) {
			return nil, chaterrors.ErrStreamTaskNotFound
		}
		return nil, err
	}

	return mapper.StreamTaskFromSchema(sch)
}

func (r *StreamTaskRepositoryImpl) DeleteById(ctx context.Context, taskId valobj.Id) error {
	return r.streamCache.DeleteTask(ctx, taskId.String())
}

func (r *StreamTaskRepositoryImpl) EmitStreamEvent(ctx context.Context, taskId valobj.Id, event *entity.StreamTaskEvent) error {
	data, err := mapper.StreamTaskEventToData(event)
	if err != nil {
		return err
	}

	if event.Id == "" {
		return errors.ErrParams.Msg("stream task event id is required")
	}

	_, err = r.streamCache.AppendEventStream(ctx, taskId.String(),
		&schema.ChatMessageStreamEvent{
			Id:   event.Id,
			Data: data,
		})
	return err
}

func (r *StreamTaskRepositoryImpl) DeleteStream(ctx context.Context, taskId valobj.Id) error {
	return r.streamCache.DeleteEventStream(ctx, taskId.String())
}

func (r *StreamTaskRepositoryImpl) SetStreamTTL(ctx context.Context, taskId valobj.Id, ttl time.Duration) error {
	return r.streamCache.SetEventStreamTTL(ctx, taskId.String(), ttl)
}

func (r *StreamTaskRepositoryImpl) BlockOnStreamEvent(
	ctx context.Context,
	taskId valobj.Id,
	opts chatrepo.BlockOnStreamEventOptions,
) ([]*entity.StreamTaskEvent, error) {
	events, err := r.streamCache.PullEventStream(ctx, taskId.String(), schema.PullEventStreamArgs{
		LastId: normalizeStreamEventId(opts.LastEventId),
		Block:  opts.Timeout,
		Count:  opts.Count,
	})
	if err != nil {
		if errors.Is(err, cache.ErrStreamNoData) {
			return nil, nil
		}
		return nil, err
	}

	results := make([]*entity.StreamTaskEvent, 0, len(events))
	for _, ev := range events {
		event, err := mapper.StreamTaskEventFromData(ev.Data)
		if err != nil {
			slog.ErrorContext(ctx, "unmarshal stream task event failed",
				slog.String("task_id", taskId.String()),
				slog.String("event_id", ev.Id),
				slog.Any("err", err),
			)
			continue
		}
		if ev.Id != "" {
			event.Id = ev.Id
		}
		results = append(results, event)
	}

	return results, nil
}

// normalizeStreamEventId 校验 Redis Stream ID 格式 <ms>-<seq>，非法时回退到 "0-0"。
// https://redis.io/docs/latest/commands/xadd/
func normalizeStreamEventId(lastId string) string {
	parts := strings.Split(lastId, "-")
	if len(parts) != 2 {
		return "0-0"
	}

	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		return "0-0"
	}
	if _, err := strconv.ParseInt(parts[1], 10, 64); err != nil {
		return "0-0"
	}

	return lastId
}
