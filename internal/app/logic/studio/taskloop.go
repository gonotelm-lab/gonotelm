package studio

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	bizartifact "github.com/gonotelm-lab/gonotelm/internal/app/biz/artifact"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/log"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/panjf2000/ants/v2"
)

const (
	defaultNumClaimers        = 10
	defaultScanInterval       = 500 * time.Millisecond
	defaultNumOfWorkGroup     = 2
	defaultNumWorkersPerGroup = 5
)

type taskLoopConfig struct {
	// 参与认领任务的协程数
	numClaimers int

	// 每轮扫描的间隔时间
	scanInterval time.Duration

	// 工作协程数量
	numOfWorkGroup     int
	numWorkersPerGroup int
}

func (c *taskLoopConfig) normalize() {
	if c.numClaimers <= 0 {
		c.numClaimers = defaultNumClaimers
	}
	if c.scanInterval <= 0 {
		c.scanInterval = defaultScanInterval
	}
	if c.numOfWorkGroup <= 0 {
		c.numOfWorkGroup = defaultNumOfWorkGroup
	}
	if c.numWorkersPerGroup <= 0 {
		c.numWorkersPerGroup = defaultNumWorkersPerGroup
	}
}

type taskLoop struct {
	ctx       context.Context
	cfg       taskLoopConfig
	closeOnce sync.Once
	close     chan struct{}
	claimers  sync.WaitGroup
	workers   sync.WaitGroup
	g         *ants.MultiPool

	taskBiz    *bizartifact.Biz
	dispatcher *taskDispatcher
}

func newTaskLoop(
	ctx context.Context,
	cfg taskLoopConfig,
	taskBiz *bizartifact.Biz,
	dispatcher *taskDispatcher,
) *taskLoop {
	cfg.normalize()

	pool, _ := ants.NewMultiPool(
		cfg.numOfWorkGroup,
		cfg.numWorkersPerGroup,
		ants.LeastTasks,
		ants.WithLogger(&log.AntsLogger{}),
	)
	return &taskLoop{
		ctx:        ctx,
		cfg:        cfg,
		g:          pool,
		close:      make(chan struct{}),
		taskBiz:    taskBiz,
		dispatcher: dispatcher,
	}
}

// 开始循环
func (t *taskLoop) start() {
	t.claimers.Add(t.cfg.numClaimers)
	for i := 0; i < t.cfg.numClaimers; i++ {
		go t.claimLoop()
	}
}

func (t *taskLoop) claimLoop() {
	claimerId := uuid.NewV4().String()
	ticker := time.NewTicker(t.cfg.scanInterval)
	defer func() {
		if err := recover(); err != nil {
			slog.ErrorContext(t.ctx, "task claim loop panic",
				slog.Any("err", err),
				slog.String("stack", string(debug.Stack())),
			)
		}
		t.claimers.Done()
		ticker.Stop()
		slog.InfoContext(t.ctx, "task claim loop stopped", slog.String("claimer", claimerId))
	}()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.close:
			return
		case <-ticker.C:
			newTask, claimed, err := t.taskBiz.TryClaimTask(t.ctx)
			if err != nil {
				slog.ErrorContext(t.ctx, "task claim loop try claim task failed", slog.Any("err", err))
				continue
			}

			if claimed {
				t.workers.Add(1)
				if err := t.g.Submit(func() {
					t.handleWork(newTask)
					t.workers.Done()
				}); err != nil {
					slog.ErrorContext(t.ctx, "task claim loop submit work failed", slog.Any("err", err))
					_ = t.taskBiz.FailTask(t.ctx, newTask.Id, newTask.RunId) // 提交任务失败的话当作失败处理
					t.workers.Done()
				}
			}
		}
	}
}

// 停止接受任务
func (t *taskLoop) stop() {
	t.closeOnce.Do(func() {
		close(t.close)
	})
}

// 关闭后等待所有任务完成
func (t *taskLoop) wait() {
	t.claimers.Wait()
	t.workers.Wait()
	t.g.ReleaseTimeout(time.Second * 10)
}

func (t *taskLoop) handleWork(task *model.ArtifactTask) {
	defer func() {
		if err := recover(); err != nil {
			slog.ErrorContext(t.ctx, "task handle work panic",
				slog.Any("err", err),
				slog.String("stack", string(debug.Stack())),
				slog.String("task_id", task.Id.String()),
				slog.String("task_status", task.Status.String()),
				slog.String("task_kind", task.Kind.String()),
			)

			// mark failed
			if err := t.taskBiz.FailTask(t.ctx, task.Id, task.RunId); err != nil {
				slog.ErrorContext(t.ctx, "task handle work fail task failed", slog.Any("err", err))
			}
		}
	}()

	ctx := pkgcontext.WithUserId(t.ctx, task.UserId)

	slog.DebugContext(ctx, "task handle work started",
		slog.String("task_id", task.Id.String()),
		slog.String("task_status", task.Status.String()),
		slog.String("task_kind", task.Kind.String()),
	)

	result, err := t.dispatcher.dispatch(ctx, task)
	if err != nil {
		slog.ErrorContext(ctx, "task handle work dispatch failed",
			slog.Any("err", err),
			slog.String("task_id", task.Id.String()),
			slog.String("task_status", task.Status.String()),
			slog.String("task_kind", task.Kind.String()),
			slog.String("task_run_id", task.RunId),
		)
		if err := t.taskBiz.FailTask(ctx, task.Id, task.RunId); err != nil {
			slog.ErrorContext(ctx, "task handle work fail task failed", slog.Any("err", err))
		}

		return
	}

	// get again
	status, err := t.taskBiz.GetTaskStatus(ctx, task.Id)
	if err != nil {
		slog.ErrorContext(ctx, "task handle work get task status failed", slog.Any("err", err))
	} else {
		if status.Cancelled() {
			slog.DebugContext(ctx, "target task is already cancelled, skip..",
				slog.String("task_id", task.Id.String()),
				slog.String("task_run_id", task.RunId),
				slog.String("task_kind", task.Kind.String()),
				slog.String("task_status", status.String()),
			)
			return
		}
	}

	if err := t.taskBiz.CompleteTask(ctx, &bizartifact.CompleteTaskCommand{
		TaskId:     task.Id,
		RunId:      task.RunId,
		Title:      result.title,
		Result:     result.result,
		ResultKind: result.resultKind,
	}); err != nil {
		slog.ErrorContext(ctx, "task handle work complete task failed", slog.Any("err", err))
		return
	}

	slog.DebugContext(ctx, "task handle work completed",
		slog.String("task_id", task.Id.String()),
		slog.String("task_kind", task.Kind.String()),
	)
}
