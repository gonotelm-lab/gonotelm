package postgres

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests do not call t.Parallel: AbortStale acts on the whole table, so
// they must not run while another test is creating running rows.

var initJobTaskSeq atomic.Int64

// newInitJobTaskID builds a unique id of the expected shape (YYYYMMDD + 4 digits).
func newInitJobTaskID() string {
	return fmt.Sprintf("%s%04d", time.Now().UTC().Format("20060102"), initJobTaskSeq.Add(1)%10000)
}

func newInitJobRun(t *testing.T, status string) *schema.InitJobRun {
	t.Helper()
	now := nowMilli()
	run := &schema.InitJobRun{
		Id:        uuid.NewV7(),
		Status:    status,
		RunMode:   "strict",
		Pod:       "test-pod",
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, testInitJobStore.CreateRun(context.Background(), run))
	return run
}

func newInitJobTask(t *testing.T, status string) *schema.InitJobTask {
	t.Helper()
	run := newInitJobRun(t, status)
	task := &schema.InitJobTask{
		TaskId:      newInitJobTaskID(),
		Description: "test task",
		RunId:       run.Id,
		Status:      status,
	}
	require.NoError(t, testInitJobStore.UpsertTaskRunning(context.Background(), task))
	if status != schema.InitJobStatusRunning {
		now := nowMilli()
		require.NoError(t, testInitJobStore.UpdateTaskStatus(context.Background(),
			&schema.InitJobTaskStatusParams{
				TaskId:     task.TaskId,
				Status:     status,
				FinishedAt: now,
				UpdatedAt:  now,
			}))
	}
	return task
}

func getInitJobRun(t *testing.T, id uuid.UUID) *schema.InitJobRun {
	t.Helper()
	var run schema.InitJobRun
	require.NoError(t, testDB.WithContext(context.Background()).Where("id = ?", id).Take(&run).Error)
	return &run
}

func getInitJobTask(t *testing.T, taskID string) *schema.InitJobTask {
	t.Helper()
	var task schema.InitJobTask
	require.NoError(t, testDB.WithContext(context.Background()).
		Where("task_id = ?", taskID).Take(&task).Error)
	return &task
}

func TestInitJobStore_AbortStaleClosesRunningRecordsOnly(t *testing.T) {
	ctx := context.Background()

	staleRun := newInitJobRun(t, schema.InitJobStatusRunning)
	staleTask := newInitJobTask(t, schema.InitJobStatusRunning)
	doneRun := newInitJobRun(t, schema.InitJobStatusSucceeded)
	doneTask := newInitJobTask(t, schema.InitJobStatusSucceeded)

	now := nowMilli()
	abortedRuns, abortedTasks, err := testInitJobStore.AbortStale(ctx, &schema.InitJobAbortStaleParams{
		RunStatus:  schema.InitJobStatusAborted,
		TaskStatus: schema.InitJobStatusAborted,
		Error:      "aborted for test",
		FinishedAt: now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, abortedRuns, int64(1))
	assert.GreaterOrEqual(t, abortedTasks, int64(1))

	// Non-parallel tests own the table while they run, so these are the rows
	// this test created.
	gotRun := getInitJobRun(t, staleRun.Id)
	assert.Equal(t, schema.InitJobStatusAborted, gotRun.Status)
	assert.Equal(t, "aborted for test", gotRun.Error)
	assert.Equal(t, now, gotRun.FinishedAt)

	gotTask := getInitJobTask(t, staleTask.TaskId)
	assert.Equal(t, schema.InitJobStatusAborted, gotTask.Status)
	assert.Equal(t, now, gotTask.FinishedAt)

	// Terminal records must not be touched.
	assert.Equal(t, schema.InitJobStatusSucceeded, getInitJobRun(t, doneRun.Id).Status)
	assert.Equal(t, schema.InitJobStatusSucceeded, getInitJobTask(t, doneTask.TaskId).Status)
}

func TestInitJobStore_UpsertTaskRunningBumpsAttemptAndRefreshesDescription(t *testing.T) {
	ctx := context.Background()
	taskID := newInitJobTaskID()

	first := nowMilli()
	firstRun := newInitJobRun(t, schema.InitJobStatusRunning)
	require.NoError(t, testInitJobStore.UpsertTaskRunning(ctx, &schema.InitJobTask{
		TaskId:      taskID,
		Description: "first description",
		RunId:       firstRun.Id,
		Status:      schema.InitJobStatusRunning,
		Attempt:     1,
		StartedAt:   first,
		CreatedAt:   first,
		UpdatedAt:   first,
	}))

	got := getInitJobTask(t, taskID)
	assert.Equal(t, int32(1), got.Attempt)
	assert.Equal(t, "first description", got.Description)
	assert.Equal(t, firstRun.Id, got.RunId)
	assert.Equal(t, first, got.CreatedAt)

	// A second start of the same task must count as a new attempt and refresh
	// the description and run, while created_at stays from the first insert.
	second := nowMilli() + 1
	secondRun := newInitJobRun(t, schema.InitJobStatusRunning)
	require.NoError(t, testInitJobStore.UpsertTaskRunning(ctx, &schema.InitJobTask{
		TaskId:      taskID,
		Description: "second description",
		RunId:       secondRun.Id,
		Status:      schema.InitJobStatusRunning,
		Attempt:     1,
		StartedAt:   second,
		CreatedAt:   second,
		UpdatedAt:   second,
	}))

	got = getInitJobTask(t, taskID)
	assert.Equal(t, int32(2), got.Attempt)
	assert.Equal(t, "second description", got.Description)
	assert.Equal(t, secondRun.Id, got.RunId)
	assert.Equal(t, second, got.StartedAt)
	assert.Equal(t, int64(0), got.FinishedAt, "a restarted task must not look finished")
	assert.Equal(t, first, got.CreatedAt, "created_at must survive re-execution")
}

func TestInitJobStore_UpdateTaskStatusAndListSucceeded(t *testing.T) {
	ctx := context.Background()

	succeeded := newInitJobTask(t, schema.InitJobStatusRunning)
	failed := newInitJobTask(t, schema.InitJobStatusRunning)

	now := nowMilli()
	require.NoError(t, testInitJobStore.UpdateTaskStatus(ctx, &schema.InitJobTaskStatusParams{
		TaskId:     succeeded.TaskId,
		Status:     schema.InitJobStatusSucceeded,
		FinishedAt: now,
		UpdatedAt:  now,
	}))
	require.NoError(t, testInitJobStore.UpdateTaskStatus(ctx, &schema.InitJobTaskStatusParams{
		TaskId:     failed.TaskId,
		Status:     schema.InitJobStatusFailed,
		Error:      "boom",
		FinishedAt: now,
		UpdatedAt:  now,
	}))

	gotSucceeded := getInitJobTask(t, succeeded.TaskId)
	assert.Equal(t, schema.InitJobStatusSucceeded, gotSucceeded.Status)
	assert.Equal(t, now, gotSucceeded.FinishedAt)

	gotFailed := getInitJobTask(t, failed.TaskId)
	assert.Equal(t, schema.InitJobStatusFailed, gotFailed.Status)
	assert.Equal(t, "boom", gotFailed.Error)
	assert.Equal(t, now, gotFailed.FinishedAt)

	ids, err := testInitJobStore.ListSucceededTaskIDs(ctx)
	require.NoError(t, err)
	assert.Contains(t, ids, succeeded.TaskId)
	assert.NotContains(t, ids, failed.TaskId)
}

func TestInitJobStore_FinishRun(t *testing.T) {
	ctx := context.Background()
	run := newInitJobRun(t, schema.InitJobStatusRunning)

	now := nowMilli()
	require.NoError(t, testInitJobStore.FinishRun(ctx, &schema.InitJobRunFinishParams{
		RunId:      run.Id,
		Status:     schema.InitJobStatusFailed,
		ExitCode:   1,
		Error:      "task 202610050001 failed",
		FinishedAt: now,
		UpdatedAt:  now,
	}))

	got := getInitJobRun(t, run.Id)
	assert.Equal(t, schema.InitJobStatusFailed, got.Status)
	assert.Equal(t, int32(1), got.ExitCode)
	assert.Equal(t, "task 202610050001 failed", got.Error)
	assert.Equal(t, now, got.FinishedAt)
	assert.Equal(t, "test-pod", got.Pod)
}
