package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InitJobStoreImpl struct{ db *gorm.DB }

var _ database.InitJobStore = &InitJobStoreImpl{}

func NewInitJobStoreImpl(db *gorm.DB) *InitJobStoreImpl { return &InitJobStoreImpl{db: db} }

// AbortStale closes records left running by a process that never finished.
func (s *InitJobStoreImpl) AbortStale(
	ctx context.Context, params *schema.InitJobAbortStaleParams,
) (int64, int64, error) {
	runs := s.db.WithContext(ctx).
		Model(&schema.InitJobRun{}).
		Where("status = ?", schema.InitJobStatusRunning).
		Updates(map[string]any{
			"status":      params.RunStatus,
			"error":       params.Error,
			"finished_at": params.FinishedAt,
			"updated_at":  params.UpdatedAt,
		})
	if runs.Error != nil {
		return 0, 0, sql.WrapErr(runs.Error)
	}

	tasks := s.db.WithContext(ctx).
		Model(&schema.InitJobTask{}).
		Where("status = ?", schema.InitJobStatusRunning).
		Updates(map[string]any{
			"status":      params.TaskStatus,
			"finished_at": params.FinishedAt,
			"updated_at":  params.UpdatedAt,
		})
	if tasks.Error != nil {
		return runs.RowsAffected, 0, sql.WrapErr(tasks.Error)
	}

	return runs.RowsAffected, tasks.RowsAffected, nil
}

func (s *InitJobStoreImpl) CreateRun(ctx context.Context, run *schema.InitJobRun) error {
	if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *InitJobStoreImpl) FinishRun(
	ctx context.Context, params *schema.InitJobRunFinishParams,
) error {
	err := s.db.WithContext(ctx).
		Model(&schema.InitJobRun{}).
		Where("id = ?", params.RunId).
		Updates(map[string]any{
			"status":      params.Status,
			"exit_code":   params.ExitCode,
			"error":       params.Error,
			"finished_at": params.FinishedAt,
			"updated_at":  params.UpdatedAt,
		}).Error
	if err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *InitJobStoreImpl) ListSucceededTaskIDs(ctx context.Context) ([]string, error) {
	var ids []string
	err := s.db.WithContext(ctx).
		Model(&schema.InitJobTask{}).
		Where("status = ?", schema.InitJobStatusSucceeded).
		Pluck("task_id", &ids).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return ids, nil
}

// UpsertTaskRunning marks the task running and bumps attempt in one statement,
// so concurrent or repeated starts cannot lose the count.
func (s *InitJobStoreImpl) UpsertTaskRunning(ctx context.Context, task *schema.InitJobTask) error {
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"description": task.Description,
			"run_id":      task.RunId,
			"status":      task.Status,
			"attempt":     gorm.Expr("initjob_tasks.attempt + 1"),
			"error":       "",
			"started_at":  task.StartedAt,
			"finished_at": 0,
			"updated_at":  task.UpdatedAt,
		}),
	}).Create(task).Error
	if err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *InitJobStoreImpl) UpdateTaskStatus(
	ctx context.Context, params *schema.InitJobTaskStatusParams,
) error {
	err := s.db.WithContext(ctx).
		Model(&schema.InitJobTask{}).
		Where("task_id = ?", params.TaskId).
		Updates(map[string]any{
			"status":      params.Status,
			"error":       params.Error,
			"finished_at": params.FinishedAt,
			"updated_at":  params.UpdatedAt,
		}).Error
	if err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
