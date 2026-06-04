package artifact

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const (
	defaultTaskExpiration = time.Minute * 30
)

var ErrTaskNotFound = errors.ErrParams.Msgf("task not found")

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

func (b *Biz) GetTask(ctx context.Context, id uuid.UUID) (*model.ArtifactTask, error) {
	task, err := b.taskStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, ErrTaskNotFound
		}

		return nil, errors.WithMessagef(err, "get artifact task failed, id=%s", id)
	}

	return model.NewArtifactTaskFrom(task), nil
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
	Result     []byte
	ResultKind model.ArtifactResultKind
}

// 设置指定的任务成功并且设置结果
func (b *Biz) CompleteTask(ctx context.Context, cmd *CompleteTaskCommand) error {
	_, err := b.taskStore.UpdateResult(ctx, cmd.TaskId, cmd.RunId,
		model.ArtifactStatusRunning.String(),
		&schema.ArtifactTaskUpdateResultParams{
			NewStatus:  model.ArtifactStatusCompleted.String(),
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

func taskRunId() string {
	return uuid.NewV4().String()
}

func getUnixMilli() int64 {
	return time.Now().UnixMilli()
}
