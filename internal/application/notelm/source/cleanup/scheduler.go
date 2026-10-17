package cleanup

import (
	"context"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// CheckpointScheduler 在删除发生时注册一次性清理检查点。
type CheckpointScheduler struct {
	client  flow.ScheduleClient
	baseURL string
	delay   time.Duration
	jitter  time.Duration

	// 单测注入
	now       func() time.Time
	randFloat func() float64
}

func NewCheckpointScheduler(
	client flow.ScheduleClient,
	baseURL string,
	delay, jitter time.Duration,
) *CheckpointScheduler {
	return &CheckpointScheduler{
		client:    client,
		baseURL:   strings.TrimRight(baseURL, "/"),
		delay:     delay,
		jitter:    jitter,
		now:       time.Now,
		randFloat: rand.Float64,
	}
}

// Enabled 没配回调地址或没有 flow 客户端时不注册。
func (s *CheckpointScheduler) Enabled() bool {
	return s != nil && s.client != nil && s.baseURL != ""
}

func (s *CheckpointScheduler) ScheduleSourceCleanup(
	ctx context.Context,
	notebookId, sourceId valobj.Id,
	keys []valobj.StoreKey,
) error {
	p := &Payload{
		Kind:       KindSource,
		NotebookId: notebookId,
		SourceId:   sourceId,
		Keys:       EncodeKeys(keys),
	}

	return s.register(ctx, p, SourceCallbackPath, KindSource+":"+notebookId.String()+":"+sourceId.String())
}

func (s *CheckpointScheduler) ScheduleNotebookCleanup(
	ctx context.Context,
	notebookId valobj.Id,
	refs []SourceRef,
) error {
	p := &Payload{Kind: KindNotebook, NotebookId: notebookId, Sources: refs}

	return s.register(ctx, p, NotebookCallbackPath, KindNotebook+":"+notebookId.String())
}

func (s *CheckpointScheduler) register(
	ctx context.Context,
	p *Payload,
	path string,
	idempotencyKey string,
) error {
	if !s.Enabled() {
		return nil
	}

	payload, err := p.Marshal()
	if err != nil {
		return err
	}

	fireAt := s.nextFireAt()
	scheduleId, err := s.client.RegisterOneShot(
		ctx,
		fireAt,
		s.baseURL+path,
		payload,
		idempotencyKey,
	)
	if err != nil {
		return errors.WithMessagef(err, "register %s checkpoint failed", p.Kind)
	}

	slog.DebugContext(ctx, "cleanup checkpoint registered",
		slog.String("kind", p.Kind),
		slog.String("schedule_id", scheduleId),
		slog.Time("fire_at", fireAt),
	)

	return nil
}

// nextFireAt = now + delay + [0, jitter)，抖动用来分散删除高峰。
func (s *CheckpointScheduler) nextFireAt() time.Time {
	at := s.now().Add(s.delay)
	if s.jitter > 0 {
		at = at.Add(time.Duration(s.randFloat() * float64(s.jitter)))
	}

	return at
}

// EncodeKeys 把 StoreKey 编成载荷里的字符串形式，跳过无效 key。
func EncodeKeys(keys []valobj.StoreKey) []string {
	if len(keys) == 0 {
		return nil
	}

	encoded := make([]string, 0, len(keys))
	for _, key := range keys {
		if key.Valid() {
			encoded = append(encoded, key.Encode())
		}
	}

	return encoded
}
