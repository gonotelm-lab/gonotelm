package postgres

import (
	"context"
	"errors"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ArtifactTaskStoreImpl struct {
	db *gorm.DB
}

var _ dal.ArtifactTaskStore = &ArtifactTaskStoreImpl{}

func NewArtifactTaskStoreImpl(db *gorm.DB) *ArtifactTaskStoreImpl {
	return &ArtifactTaskStoreImpl{db: db}
}

func (a *ArtifactTaskStoreImpl) Create(ctx context.Context, task *schema.ArtifactTask) error {
	if err := a.db.WithContext(ctx).Create(task).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (a *ArtifactTaskStoreImpl) GetById(ctx context.Context, id dal.Id) (*schema.ArtifactTask, error) {
	var task schema.ArtifactTask
	if err := a.db.WithContext(ctx).
		Where("id = ?", id).
		Take(&task).Error; err != nil {
		return nil, sql.WrapErr(err)
	}

	return &task, nil
}

func (a *ArtifactTaskStoreImpl) PageListByNotebookId(
	ctx context.Context,
	notebookId dal.Id,
	cursor dal.Id, limit int,
) ([]*schema.ArtifactTask, error) {
	var tasks []*schema.ArtifactTask
	if err := a.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Where("id > ?", cursor).
		Order("id ASC").
		Limit(limit).
		Find(&tasks).Error; err != nil {
		return nil, sql.WrapErr(err)
	}

	return tasks, nil
}

func (a *ArtifactTaskStoreImpl) Claim(
	ctx context.Context,
	oldStatus string,
	now int64,
	params *schema.ArtifactTaskClaimParams,
) (*schema.ArtifactTask, bool, error) {
	if params.Mode == 0 {
		return a.claimWithSkipLockMode(ctx, oldStatus, now, params)
	}

	return a.claimWithVersionLockMode(ctx, oldStatus, now, params)
}

func (a *ArtifactTaskStoreImpl) claimWithSkipLockMode(
	ctx context.Context,
	oldStatus string,
	lastExpiredAt int64,
	params *schema.ArtifactTaskClaimParams,
) (*schema.ArtifactTask, bool, error) {
	var (
		task    schema.ArtifactTask
		claimed = false
	)

	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. select * from %s where xxx for update skip locked
		tx = tx.WithContext(ctx)
		err := tx.Clauses(clause.Locking{
			Strength: clause.LockingStrengthUpdate,    // UPDATE
			Options:  clause.LockingOptionsSkipLocked, // SKIP LOCKED
		}).Where("expired_at > ?", lastExpiredAt).
			Where("status = ?", oldStatus).
			Order("created_at ASC").
			Order("id ASC").
			Limit(1).
			Take(&task).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}

			return sql.WrapErr(err)
		}

		// 2. update selected row
		result := tx.Model(&task).
			Where("id = ?", task.Id).
			Where("status = ?", task.Status).
			Where("lock_no = ?", task.LockNo).
			Updates(map[string]any{
				"status":     params.NewStatus,
				"run_id":     params.RunId,
				"updated_at": params.UpdatedAt,
				"lock_no":    task.LockNo + 1,
			})
		if result.Error != nil {
			return sql.WrapErr(result.Error)
		}
		if result.RowsAffected != 0 {
			claimed = true
		}

		return nil
	})
	if err != nil {
		return nil, false, err
	}

	if !claimed {
		return nil, false, nil
	}

	return &task, claimed, nil
}

func (a *ArtifactTaskStoreImpl) claimWithVersionLockMode(
	ctx context.Context,
	oldStatus string,
	lastExpiredAt int64,
	params *schema.ArtifactTaskClaimParams,
) (*schema.ArtifactTask, bool, error) {
	// 1. get first
	var task schema.ArtifactTask
	// select * from %s where status = :old_status order by created_at asc, id asc limit 1
	query := a.db.WithContext(ctx).
		Model(&schema.ArtifactTask{}).
		Where("expired_at > ?", lastExpiredAt).
		Where("status = ?", oldStatus).
		Order("created_at ASC").
		Order("id ASC").
		Limit(1)
	if err := query.Take(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, sql.WrapErr(err)
	}

	// 2. then claim this task by update status to new status and set run_id
	// update %s
	// set status = :new_status, run_id = :run_id, updated_at = :updated_at, lock_no = lock_no + 1
	// where id = :id and status = :old_status and lock_no = :lock_no
	result := a.db.WithContext(ctx).
		Model(&schema.ArtifactTask{}).
		Where("id = ?", task.Id).
		Where("status = ?", task.Status).
		Where("lock_no = ?", task.LockNo).
		Updates(map[string]any{
			"status":     params.NewStatus,
			"lock_no":    task.LockNo + 1,
			"run_id":     params.RunId,
			"updated_at": params.UpdatedAt,
		})
	if result.Error != nil {
		return nil, false, sql.WrapErr(result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, false, nil
	}

	return &task, true, nil
}

func (a *ArtifactTaskStoreImpl) SetStatus(
	ctx context.Context,
	id dal.Id,
	newStatus string,
	updatedAt int64,
) error {
	if err := a.db.WithContext(ctx).
		Model(&schema.ArtifactTask{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     newStatus,
			"updated_at": updatedAt,
		}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (a *ArtifactTaskStoreImpl) UpdateStatus(
	ctx context.Context,
	id dal.Id,
	runId string,
	oldStatus string,
	params *schema.ArtifactTaskUpdateStatusParams,
) (bool, error) {
	// update %s
	// set status = :new_status, updated_at = :updated_at
	// where id = :id and status = :old_status and run_id = :run_id
	query := a.db.WithContext(ctx).
		Model(&schema.ArtifactTask{}).
		Where("id = ?", id).
		Where("status = ?", oldStatus).
		Where("run_id = ?", runId).
		Updates(map[string]any{
			"status":     params.NewStatus,
			"updated_at": params.UpdatedAt,
		})
	if query.Error != nil {
		return false, sql.WrapErr(query.Error)
	}

	if query.RowsAffected == 0 {
		return false, nil
	}

	return true, nil
}

func (a *ArtifactTaskStoreImpl) UpdateResult(
	ctx context.Context,
	id dal.Id,
	runId string,
	oldStatus string,
	params *schema.ArtifactTaskUpdateResultParams,
) (bool, error) {
	// update %s
	// set result = :result, result_kind = :result_kind, updated_at = :updated_at
	// where id = :id and status = :old_status and run_id = :run_id
	query := a.db.WithContext(ctx).
		Model(&schema.ArtifactTask{}).
		Where("id = ?", id).
		Where("status = ?", oldStatus).
		Where("run_id = ?", runId).
		Updates(map[string]any{
			"status":      params.NewStatus,
			"result":      params.Result,
			"result_kind": params.ResultKind,
			"updated_at":  params.UpdatedAt,
		})
	if query.Error != nil {
		return false, sql.WrapErr(query.Error)
	}

	if query.RowsAffected == 0 {
		return false, nil
	}

	return true, nil
}

func (a *ArtifactTaskStoreImpl) DeleteById(ctx context.Context, id dal.Id) error {
	if err := a.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&schema.ArtifactTask{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (a *ArtifactTaskStoreImpl) SetExpiredTasksStatus(
	ctx context.Context,
	ids []dal.Id,
	newStatus string,
	updatedAt int64,
	now int64,
) error {
	if err := a.db.WithContext(ctx).
		Model(&schema.ArtifactTask{}).
		Where("id IN ?", ids).
		Where("expired_at <= ?", now).
		Updates(map[string]any{
			"status":     newStatus,
			"updated_at": updatedAt,
		}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (a *ArtifactTaskStoreImpl) PageListExpiredTasks(
	ctx context.Context,
	cursor dal.Id,
	limit int,
	now int64,
) ([]*schema.ArtifactTask, error) {
	var tasks []*schema.ArtifactTask
	if err := a.db.WithContext(ctx).
		Model(&schema.ArtifactTask{}).
		Where("expired_at <= ?", now).
		Where("id > ?", cursor).
		Order("id ASC").
		Limit(limit).
		Find(&tasks).Error; err != nil {
		return nil, sql.WrapErr(err)
	}

	return tasks, nil
}
