package initjob

import "context"

// Status is the state of a run or a task.
type Status string

const (
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	// StatusAborted marks records left running by a killed process; the next
	// startup's AbortStale closes them.
	StatusAborted Status = "aborted"
)

// Recorder is the framework's only persistence dependency. The Postgres
// implementation lives under internal/infrastructure.
type Recorder interface {
	// AbortStale marks stale running runs and tasks aborted; called at startup.
	AbortStale(ctx context.Context) error

	// StartRun creates a run record and returns its id.
	StartRun(ctx context.Context, mode Mode, pod string) (runID string, err error)

	// FinishRun writes the run's final status, exit code and error summary.
	FinishRun(ctx context.Context, runID string, status Status, exitCode int, errText string) error

	// SucceededTaskIDs returns the ids of every task that succeeded historically.
	SucceededTaskIDs(ctx context.Context) (map[string]struct{}, error)

	// MarkRunning sets the task to running, stores its description and
	// increments attempt.
	MarkRunning(ctx context.Context, runID string, taskID string, description string) error

	// MarkSucceeded sets the task to succeeded.
	MarkSucceeded(ctx context.Context, runID string, taskID string) error

	// MarkFailed sets the task to failed and stores the error summary.
	MarkFailed(ctx context.Context, runID string, taskID string, errText string) error
}

// Mode is the failure policy.
type Mode string

const (
	// ModeStrict stops at the first failed task and exits non-zero.
	ModeStrict Mode = "strict"
	// ModeLoose keeps going after a failure and still exits non-zero if any failed.
	ModeLoose Mode = "loose"
)

// Valid reports whether the mode is recognized.
func (m Mode) Valid() bool { return m == ModeStrict || m == ModeLoose }
