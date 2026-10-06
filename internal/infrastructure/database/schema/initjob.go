package schema

import "github.com/gonotelm-lab/gonotelm/pkg/uuid"

// Initjob status values stored in the status columns. The recorder adapter
// maps pkg/initjob.Status onto these, so the two vocabularies stay in sync.
const (
	InitJobStatusRunning   = "running"
	InitJobStatusSucceeded = "succeeded"
	InitJobStatusFailed    = "failed"
	InitJobStatusAborted   = "aborted"
)

// InitJobRun mirrors one row of initjob_runs.
type InitJobRun struct {
	Id         uuid.UUID `gorm:"column:id;primaryKey"`
	Status     string    `gorm:"column:status"`
	RunMode    string    `gorm:"column:run_mode"`
	Pod        string    `gorm:"column:pod"`
	ExitCode   int32     `gorm:"column:exit_code"`
	Error      string    `gorm:"column:error"`
	StartedAt  int64     `gorm:"column:started_at"`
	FinishedAt int64     `gorm:"column:finished_at"`
	CreatedAt  int64     `gorm:"column:created_at"`
	UpdatedAt  int64     `gorm:"column:updated_at"`
}

func (InitJobRun) TableName() string { return "initjob_runs" }

// InitJobTask mirrors one row of initjob_tasks.
type InitJobTask struct {
	TaskId      string    `gorm:"column:task_id;primaryKey"`
	Description string    `gorm:"column:description"`
	RunId       uuid.UUID `gorm:"column:run_id"`
	Status      string    `gorm:"column:status"`
	Attempt     int32     `gorm:"column:attempt"`
	Error       string    `gorm:"column:error"`
	StartedAt   int64     `gorm:"column:started_at"`
	FinishedAt  int64     `gorm:"column:finished_at"`
	CreatedAt   int64     `gorm:"column:created_at"`
	UpdatedAt   int64     `gorm:"column:updated_at"`
}

func (InitJobTask) TableName() string { return "initjob_tasks" }

// InitJobRunFinishParams closes a run.
type InitJobRunFinishParams struct {
	RunId      uuid.UUID
	Status     string
	ExitCode   int32
	Error      string
	FinishedAt int64
	UpdatedAt  int64
}

// InitJobTaskStatusParams updates one task's latest result.
type InitJobTaskStatusParams struct {
	TaskId     string
	Status     string
	Error      string
	FinishedAt int64
	UpdatedAt  int64
}

// InitJobAbortStaleParams closes records left running by a killed process.
type InitJobAbortStaleParams struct {
	RunStatus  string
	TaskStatus string
	Error      string
	FinishedAt int64
	UpdatedAt  int64
}
