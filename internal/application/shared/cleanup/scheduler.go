package cleanup

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkguuid "github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const scheduleNameReqIdLen = 8

// checkpointScheduler 在删除发生时注册一次性清理检查点。
type checkpointScheduler struct {
	client flow.ScheduleClient
	cfg    Config

	// 单测注入
	now       func() time.Time
	randFloat func() float64
}

func newCheckpointScheduler(client flow.ScheduleClient, cfg Config) *checkpointScheduler {
	cfg.CallbackBaseURL = strings.TrimRight(cfg.CallbackBaseURL, "/")

	return &checkpointScheduler{
		client:    client,
		cfg:       cfg,
		now:       time.Now,
		randFloat: rand.Float64,
	}
}

const (
	actionDeleteSource   = "del-source"
	actionDeleteNotebook = "del-notebook"
	actionDeleteArtifact = "del-artifact"
	actionDeleteCover    = "del-cover"
)

func (s *checkpointScheduler) ScheduleSourceCleanup(
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
	key := KindSource + ":" + notebookId.String() + ":" + sourceId.String()
	name := scheduleName(ctx, actionDeleteSource, sourceId)

	return s.register(ctx, p, SourceCallbackPath, key, name)
}

func (s *checkpointScheduler) scheduleNotebookCleanup(
	ctx context.Context,
	notebookId valobj.Id,
	sources []SourceRef,
	artifacts []ArtifactRef,
) error {
	p := &Payload{Kind: KindNotebook, NotebookId: notebookId, Sources: sources, Artifacts: artifacts}
	key := KindNotebook + ":" + notebookId.String()
	name := scheduleName(ctx, actionDeleteNotebook, notebookId)

	return s.register(ctx, p, NotebookCallbackPath, key, name)
}

func (s *checkpointScheduler) ScheduleArtifactCleanup(
	ctx context.Context,
	notebookId, artifactId valobj.Id,
	keys []valobj.StoreKey,
) error {
	p := &Payload{
		Kind:       KindArtifact,
		NotebookId: notebookId,
		ArtifactId: artifactId,
		Keys:       EncodeKeys(keys),
	}
	key := KindArtifact + ":" + notebookId.String() + ":" + artifactId.String()
	name := scheduleName(ctx, actionDeleteArtifact, artifactId)

	return s.register(ctx, p, ArtifactCallbackPath, key, name)
}

// ScheduleCoverCleanup 注册封面在途 key 的清理检查点；fireDelay 由调用方传 entity.CoverCleanupDelay。
func (s *checkpointScheduler) ScheduleCoverCleanup(
	ctx context.Context,
	notebookId valobj.Id,
	keys []valobj.StoreKey,
	fireDelay time.Duration,
) error {
	nonce, err := pkguuid.NewRandomURLToken(8)
	if err != nil {
		return errors.WithMessage(err, "generate cover cleanup nonce failed")
	}

	p := &Payload{Kind: KindCover, NotebookId: notebookId, Keys: EncodeKeys(keys)}
	if len(p.Keys) == 0 {
		return errors.ErrParams.Msg("cover cleanup without valid keys")
	}

	// key 带 nonce：每次改挂封面都注册独立检查点
	idempotencyKey := KindCover + ":" + notebookId.String() + ":" + p.Keys[0] + ":" + nonce
	name := scheduleName(ctx, actionDeleteCover, notebookId)

	return s.registerAt(ctx, p, CoverCallbackPath, idempotencyKey, name, s.now().Add(fireDelay))
}

// name = userid+action+resourceid@reqid，reqid 只取前 8 位
func scheduleName(ctx context.Context, action string, resourceId valobj.Id) string {
	userId := pkgcontext.GetUserId(ctx)
	reqId := pkgcontext.GetReqId(ctx)
	if reqId.IsZero() {
		return fmt.Sprintf("%s+%s+%s", userId, action, resourceId)
	}

	reqIdStr := reqId.String()
	if len(reqIdStr) > scheduleNameReqIdLen {
		reqIdStr = reqIdStr[:scheduleNameReqIdLen]
	}

	return fmt.Sprintf("%s+%s+%s@%s", userId, action, resourceId, reqIdStr)
}

func (s *checkpointScheduler) register(
	ctx context.Context,
	p *Payload,
	path string,
	idempotencyKey string,
	name string,
) error {
	return s.registerAt(ctx, p, path, idempotencyKey, name, s.nextFireAt())
}

// registerAt 注册回调；fireAt 由调用方决定（默认 nextFireAt = now + delay + jitter）。
func (s *checkpointScheduler) registerAt(
	ctx context.Context,
	p *Payload,
	path string,
	idempotencyKey string,
	name string,
	fireAt time.Time,
) error {
	payload, err := p.Marshal()
	if err != nil {
		return err
	}

	scheduleId, err := s.client.RegisterOneShot(
		ctx,
		fireAt,
		s.cfg.CallbackBaseURL+path,
		payload,
		idempotencyKey,
		name,
	)
	if err != nil {
		return errors.WithMessagef(err, "register %s checkpoint failed", p.Kind)
	}

	slog.DebugContext(ctx, "cleanup checkpoint registered",
		slog.String("kind", p.Kind),
		slog.String("name", name),
		slog.String("schedule_id", scheduleId),
		slog.Time("fire_at", fireAt),
	)

	return nil
}

// nextFireAt = now + delay + [0, jitter)
func (s *checkpointScheduler) nextFireAt() time.Time {
	at := s.now().Add(s.cfg.Delay)
	if s.cfg.Jitter > 0 {
		at = at.Add(time.Duration(s.randFloat() * float64(s.cfg.Jitter)))
	}

	return at
}

// EncodeKeys 编码 StoreKey，跳过无效 key。
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
