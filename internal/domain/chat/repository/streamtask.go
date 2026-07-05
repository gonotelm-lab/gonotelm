package repository

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
)

// 消息流任务
type StreamTaskRepository interface {
	StreamTaskEventRepository

	Save(ctx context.Context, task *entity.StreamTask) error
	FindById(ctx context.Context, taskId valobj.Id) (*entity.StreamTask, error)
	DeleteById(ctx context.Context, taskId valobj.Id) error
}

type BlockOnStreamEventOptions struct {
	LastEventId string
	Timeout     time.Duration
	Count       int
}

// 消息流任务事件
type StreamTaskEventRepository interface {
	EmitStreamEvent(ctx context.Context, event *entity.StreamTaskEvent) error
	DeleteStream(ctx context.Context, taskId valobj.Id) error
	SetStreamTTL(ctx context.Context, taskId valobj.Id, ttl time.Duration) error
	BlockOnStreamEvent(ctx context.Context, taskId valobj.Id, opts BlockOnStreamEventOptions) ([]*entity.StreamTaskEvent, error)
}
