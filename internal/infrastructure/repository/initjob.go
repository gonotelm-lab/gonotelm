package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/initjob"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const initJobAbortedError = "aborted: process terminated before completion"

// InitJobRecorderImpl adapts the row-shaped InitJobStore to initjob.Recorder.
// It owns the framework vocabulary (Status/Mode) plus id and timestamp
// generation, which is what keeps the store free of framework types.
type InitJobRecorderImpl struct {
	store database.InitJobStore
	now   func() int64
}

var _ initjob.Recorder = &InitJobRecorderImpl{}

func NewInitJobRecorder(store database.InitJobStore) initjob.Recorder {
	return &InitJobRecorderImpl{
		store: store,
		now:   func() int64 { return time.Now().UnixMilli() },
	}
}

func (r *InitJobRecorderImpl) AbortStale(ctx context.Context) error {
	now := r.now()
	runs, tasks, err := r.store.AbortStale(ctx, &schema.InitJobAbortStaleParams{
		RunStatus:  schema.InitJobStatusAborted,
		TaskStatus: schema.InitJobStatusAborted,
		Error:      initJobAbortedError,
		FinishedAt: now,
		UpdatedAt:  now,
	})
	if err != nil {
		return err
	}
	if runs > 0 || tasks > 0 {
		slog.WarnContext(ctx, "initjob aborted stale records",
			slog.Int64("runs", runs), slog.Int64("tasks", tasks))
	}
	return nil
}

func (r *InitJobRecorderImpl) StartRun(
	ctx context.Context, mode initjob.Mode, pod string,
) (string, error) {
	now := r.now()
	run := &schema.InitJobRun{
		Id:        uuid.NewV7(),
		Status:    schema.InitJobStatusRunning,
		RunMode:   string(mode),
		Pod:       pod,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.store.CreateRun(ctx, run); err != nil {
		return "", err
	}
	return run.Id.String(), nil
}

func (r *InitJobRecorderImpl) FinishRun(
	ctx context.Context, runID string, status initjob.Status, exitCode int, errText string,
) error {
	id, err := parseInitJobRunID(runID)
	if err != nil {
		return err
	}
	storeStatus, err := initJobStoreStatus(status)
	if err != nil {
		return err
	}

	now := r.now()
	return r.store.FinishRun(ctx, &schema.InitJobRunFinishParams{
		RunId:      id,
		Status:     storeStatus,
		ExitCode:   int32(exitCode),
		Error:      errText,
		FinishedAt: now,
		UpdatedAt:  now,
	})
}

func (r *InitJobRecorderImpl) SucceededTaskIDs(ctx context.Context) (map[string]struct{}, error) {
	ids, err := r.store.ListSucceededTaskIDs(ctx)
	if err != nil {
		return nil, err
	}

	succeeded := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		succeeded[id] = struct{}{}
	}
	return succeeded, nil
}

func (r *InitJobRecorderImpl) MarkRunning(
	ctx context.Context, runID string, taskID string, description string,
) error {
	id, err := parseInitJobRunID(runID)
	if err != nil {
		return err
	}

	now := r.now()
	return r.store.UpsertTaskRunning(ctx, &schema.InitJobTask{
		TaskId:      taskID,
		Description: description,
		RunId:       id,
		Status:      schema.InitJobStatusRunning,
		Attempt:     1, // the store turns this into attempt + 1 when the row exists
		StartedAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

// MarkSucceeded closes the task; run_id already points at the current run.
func (r *InitJobRecorderImpl) MarkSucceeded(ctx context.Context, runID string, taskID string) error {
	return r.markTask(ctx, taskID, initjob.StatusSucceeded, "")
}

func (r *InitJobRecorderImpl) MarkFailed(
	ctx context.Context, runID string, taskID string, errText string,
) error {
	return r.markTask(ctx, taskID, initjob.StatusFailed, errText)
}

func (r *InitJobRecorderImpl) markTask(
	ctx context.Context, taskID string, status initjob.Status, errText string,
) error {
	storeStatus, err := initJobStoreStatus(status)
	if err != nil {
		return err
	}

	now := r.now()
	return r.store.UpdateTaskStatus(ctx, &schema.InitJobTaskStatusParams{
		TaskId:     taskID,
		Status:     storeStatus,
		Error:      errText,
		FinishedAt: now,
		UpdatedAt:  now,
	})
}

func parseInitJobRunID(runID string) (uuid.UUID, error) {
	id, err := uuid.ParseString(runID)
	if err != nil {
		return uuid.EmptyUUID(), fmt.Errorf("initjob: parse run id %q: %w", runID, err)
	}
	return id, nil
}

// initJobStoreStatus maps the framework vocabulary onto the stored one, so
// drift between the two surfaces as an error instead of a wrong row.
func initJobStoreStatus(status initjob.Status) (string, error) {
	switch status {
	case initjob.StatusRunning:
		return schema.InitJobStatusRunning, nil
	case initjob.StatusSucceeded:
		return schema.InitJobStatusSucceeded, nil
	case initjob.StatusFailed:
		return schema.InitJobStatusFailed, nil
	case initjob.StatusAborted:
		return schema.InitJobStatusAborted, nil
	default:
		return "", fmt.Errorf("initjob: unknown status %q", status)
	}
}
