package artifact

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const (
	defaultTaskExpiration = time.Minute * 30
)

var (
	ErrTaskNotFound          = errors.ErrParams.Msgf("artifact task not found")
	ErrCantDeleteRunningTask = errors.ErrParams.Msgf("cannot delete running task")
)

type Biz struct {
	taskStore dal.ArtifactTaskStore
}

func New(taskStore dal.ArtifactTaskStore) *Biz {
	return &Biz{taskStore: taskStore}
}

type CreateTaskCommand struct {
	NotebookId uuid.UUID
	Kind       model.ArtifactKind
	Payload    []byte
	UserId     string
}

// 创建任务 返回任务id
func (b *Biz) CreateTask(ctx context.Context, cmd *CreateTaskCommand) (uuid.UUID, error) {
	taskId := uuid.NewV7()
	now := time.Now()
	expiredAt := now.Add(defaultTaskExpiration)

	err := b.taskStore.Create(ctx, &schema.ArtifactTask{
		Id:         taskId,
		NotebookId: cmd.NotebookId,
		Kind:       cmd.Kind.String(),
		Status:     model.ArtifactStatusPending.String(),
		UserId:     cmd.UserId,
		CreatedAt:  now.UnixMilli(),
		UpdatedAt:  now.UnixMilli(),
		ExpiredAt:  expiredAt.UnixMilli(),
		Payload:    cmd.Payload,
	})
	if err != nil {
		return uuid.EmptyUUID(), errors.WithMessagef(err,
			"create artifact task failed, notebook_id=%s", cmd.NotebookId)
	}

	return taskId, nil
}

func (b *Biz) DeleteTask(ctx context.Context, id uuid.UUID) error {
	err := b.taskStore.DeleteById(ctx, id)
	if err != nil {
		return errors.WithMessagef(err,
			"delete artifact task failed, id=%s", id)
	}

	return nil
}

func (b *Biz) DeleteNotRunningTask(
	ctx context.Context,
	id uuid.UUID,
) error {
	ok, err := b.taskStore.DeleteByIdAndNotStatus(ctx, id, model.ArtifactStatusRunning.String())
	if err != nil {
		return errors.WithMessagef(err,
			"delete not running artifact task failed, id=%s", id)
	}
	if !ok {
		return ErrCantDeleteRunningTask
	}

	return nil
}

func (b *Biz) GetTask(ctx context.Context, id uuid.UUID) (*model.ArtifactTask, error) {
	task, err := b.taskStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, ErrTaskNotFound
		}

		return nil, errors.WithMessagef(err, "get artifact task failed, id=%s", id)
	}

	ret := model.NewArtifactTaskFrom(task)
	b.lazySetTaskExpiredStatus(ctx, []*model.ArtifactTask{ret})

	return ret, nil
}

func (b *Biz) GetTaskStatus(ctx context.Context, id uuid.UUID) (model.ArtifactStatus, error) {
	status, err := b.taskStore.GetStatusById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return "", ErrTaskNotFound
		}

		return "", errors.WithMessagef(err, "get artifact task status failed, id=%s", id)
	}

	return model.ArtifactStatus(status), nil
}

type ListTasksByNotebookQuery struct {
	NotebookId uuid.UUID
	Limit      int
	Offset     int
}

func (b *Biz) ListTasksByNotebook(
	ctx context.Context,
	query *ListTasksByNotebookQuery,
) ([]*model.ArtifactTask, error) {
	rows, err := b.taskStore.ListByNotebookId(
		ctx,
		query.NotebookId,
		query.Limit,
		query.Offset,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "list tasks by notebook failed")
	}

	tasks := make([]*model.ArtifactTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, model.NewArtifactTaskFrom(row))
	}
	b.lazySetTaskExpiredStatus(ctx, tasks)

	return tasks, nil
}

func (b *Biz) TryClaimTask(ctx context.Context) (*model.ArtifactTask, bool, error) {
	var (
		oldStatus = model.ArtifactStatusPending.String()
		newStatus = model.ArtifactStatusRunning.String()
		updatedAt = getUnixMilli()
		runId     = taskRunId()
	)
	task, claimed, err := b.taskStore.Claim(
		ctx,
		oldStatus,
		getUnixMilli(),
		&schema.ArtifactTaskClaimParams{
			NewStatus: newStatus,
			UpdatedAt: updatedAt,
			RunId:     runId,
		},
	)
	if err != nil {
		return nil, false, errors.WithMessagef(err, "claim artifact task failed")
	}
	if !claimed {
		return nil, false, nil
	}

	return model.NewArtifactTaskFrom(task), claimed, nil
}

// 令指定的任务失败 设置status=failed
func (b *Biz) FailTask(ctx context.Context, taskId uuid.UUID, runId string) error {
	_, err := b.taskStore.UpdateStatus(ctx, taskId, runId,
		model.ArtifactStatusRunning.String(),
		&schema.ArtifactTaskUpdateStatusParams{
			NewStatus: model.ArtifactStatusFailed.String(),
			UpdatedAt: getUnixMilli(),
		},
	)
	if err != nil {
		return errors.WithMessagef(err, "make artifact task failure error, task_id=%s, run_id=%s", taskId, runId)
	}

	return nil
}

type CompleteTaskCommand struct {
	TaskId     uuid.UUID
	RunId      string
	Title      string
	Result     []byte
	ResultKind model.ArtifactResultKind
}

// 设置指定的任务成功并且设置结果
func (b *Biz) CompleteTask(ctx context.Context, cmd *CompleteTaskCommand) error {
	title :=pkgstring.TruncateRune(cmd.Title, constants.MaxArtifactTitleLength)
	_, err := b.taskStore.UpdateResult(ctx, cmd.TaskId, cmd.RunId,
		model.ArtifactStatusRunning.String(),
		&schema.ArtifactTaskUpdateResultParams{
			NewStatus:  model.ArtifactStatusCompleted.String(),
			Title:      title,
			Result:     cmd.Result,
			ResultKind: cmd.ResultKind.String(),
			UpdatedAt:  getUnixMilli(),
		},
	)
	if err != nil {
		return errors.WithMessagef(
			err,
			"complete artifact task failed, task_id=%s, run_id=%s",
			cmd.TaskId, cmd.RunId,
		)
	}

	return nil
}

func (b *Biz) GetArtifactTaskUser(ctx context.Context, taskId uuid.UUID) (string, error) {
	task, err := b.GetTask(ctx, taskId)
	if err != nil {
		return "", errors.WithMessagef(err, "get artifact task user failed, task_id=%s", taskId)
	}

	return task.UserId, nil
}

func (b *Biz) RetryTask(ctx context.Context, taskId uuid.UUID) error {
	// 将status重置为
	// 重置状态+重置过期时间
	now := time.Now()
	expiredAt := now.Add(defaultTaskExpiration)
	err := b.taskStore.SetStatus(ctx,
		taskId,
		model.ArtifactStatusPending.String(),
		[]string{model.ArtifactStatusFailed.String(), model.ArtifactStatusCancelled.String()},
		now.UnixMilli(),
		expiredAt.UnixMilli(),
	)
	if err != nil {
		return errors.WithMessagef(err, "reset artifact task failed, task_id=%s", taskId)
	}

	return nil
}

// 取消任务
func (b *Biz) CancelRunningTask(ctx context.Context, taskId uuid.UUID) error {
	// 直接设置status
	now := time.Now()
	err := b.taskStore.SetStatus(
		ctx,
		taskId,
		model.ArtifactStatusCancelled.String(),
		[]string{model.ArtifactStatusRunning.String()},
		now.UnixMilli(),
		-1,
	)
	if err != nil {
		return errors.WithMessagef(err, "cancel artifact task failed, task_id=%s", taskId)
	}

	return nil
}

func (b *Biz) lazySetTaskExpiredStatus(
	ctx context.Context,
	tasks []*model.ArtifactTask,
) {
	now := getUnixMilli()
	expiredTasks := make([]*model.ArtifactTask, 0, len(tasks))
	for _, task := range tasks {
		if (task.Status.Running() || task.Status.Pending()) && task.ExpiredAt <= now {
			expiredTasks = append(expiredTasks, task)
		}
	}

	if len(expiredTasks) == 0 {
		return
	}

	for _, task := range expiredTasks {
		task.Status = model.ArtifactStatusExpired
	}

	// reset task status to expired in backgound
	newCtx := context.WithoutCancel(ctx)
	safe.Go(newCtx, func() {
		ids := make([]dal.Id, 0, len(expiredTasks))
		for _, task := range expiredTasks {
			ids = append(ids, task.Id)
		}
		err := b.taskStore.SetExpiredTasksStatus(
			newCtx,
			ids,
			model.ArtifactStatusExpired.String(),
			now,
			now,
		)
		if err != nil {
			slog.ErrorContext(newCtx, "set expired tasks status failed", "error", err)
		}
	})
}
