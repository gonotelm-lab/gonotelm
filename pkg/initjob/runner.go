package initjob

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"strings"
	"unicode/utf8"
)

// taskIDLen is YYYYMMDD(8) + NNNN(4).
const taskIDLen = 12

// maxErrorTextLen caps the stored error summary; the full error stays in logs.
const maxErrorTextLen = 4096

// Options tunes a Runner.
type Options struct {
	// Mode is the failure policy; the zero value defaults to ModeStrict.
	Mode Mode

	// ForceTaskIDs re-runs tasks that already succeeded. For development and
	// manual repair; keep it empty in production.
	ForceTaskIDs []string

	// Pod identifies this run, usually os.Hostname() (the Pod name on k8s).
	Pod string
}

// Runner executes a set of tasks in ascending ID order.
type Runner struct {
	tasks  []Task
	descs  map[string]string
	rec    Recorder
	opts   Options
	forced map[string]struct{}
}

// NewRunner validates and freezes the task set. Every check happens at
// startup so an invalid registry never fails halfway through a run.
func NewRunner(tasks []Task, rec Recorder, opts Options) (*Runner, error) {
	if rec == nil {
		return nil, fmt.Errorf("initjob: recorder is nil")
	}
	if opts.Mode == "" {
		opts.Mode = ModeStrict
	}
	if !opts.Mode.Valid() {
		return nil, fmt.Errorf("initjob: unknown mode %q", opts.Mode)
	}

	sorted := make([]Task, len(tasks))
	copy(sorted, tasks)

	seen := make(map[string]struct{}, len(sorted))
	descs := make(map[string]string, len(sorted))
	for i, task := range sorted {
		if task == nil {
			return nil, fmt.Errorf("initjob: task at index %d is nil", i)
		}
		id := task.ID()
		if !validTaskID(id) {
			return nil, fmt.Errorf("initjob: task id %q is invalid, want %d digits (YYYYMMDDNNNN)", id, taskIDLen)
		}
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("initjob: duplicated task id %q", id)
		}
		seen[id] = struct{}{}
		descs[id] = safeDescription(task)
	}

	// Execution order comes from the ID, not the registration order, so
	// rebuilding the registry cannot silently reorder tasks.
	slices.SortFunc(sorted, func(a, b Task) int { return strings.Compare(a.ID(), b.ID()) })

	forced := make(map[string]struct{}, len(opts.ForceTaskIDs))
	for _, id := range opts.ForceTaskIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !validTaskID(id) {
			slog.Warn("initjob: ignoring malformed force task id", "task_id", id)
			continue
		}
		if _, ok := seen[id]; !ok {
			slog.Warn("initjob: force task id does not match any registered task", "task_id", id)
		}
		forced[id] = struct{}{}
	}

	return &Runner{tasks: sorted, descs: descs, rec: rec, opts: opts, forced: forced}, nil
}

// attrs are the standard log attributes of a task.
func (r *Runner) attrs(id string) []any {
	return []any{slog.String("task_id", id), slog.String("description", r.descs[id])}
}

// TaskIDs returns the task ids in execution order.
func (r *Runner) TaskIDs() []string {
	ids := make([]string, len(r.tasks))
	for i, task := range r.tasks {
		ids[i] = task.ID()
	}
	return ids
}

// TaskInfo identifies a registered task.
type TaskInfo struct {
	ID          string
	Description string
}

// Manifest returns the registered tasks in execution order, so callers can
// print what is going to run before anything runs.
func (r *Runner) Manifest() []TaskInfo {
	infos := make([]TaskInfo, len(r.tasks))
	for i, task := range r.tasks {
		infos[i] = TaskInfo{ID: task.ID(), Description: r.descs[task.ID()]}
	}
	return infos
}

// Run executes one round. A non-nil return means the round failed or was
// aborted, so the caller must exit non-zero: that is what makes k8s retry the
// whole round and, via the success records, only the unfinished tasks.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.rec.AbortStale(ctx); err != nil {
		return fmt.Errorf("initjob: abort stale records: %w", err)
	}

	runID, err := r.rec.StartRun(ctx, r.opts.Mode, r.opts.Pod)
	if err != nil {
		return fmt.Errorf("initjob: start run: %w", err)
	}
	slog.InfoContext(ctx, "initjob run started",
		slog.String("run_id", runID),
		slog.String("mode", string(r.opts.Mode)),
		slog.String("pod", r.opts.Pod),
		slog.Int("tasks", len(r.tasks)),
		slog.Int("forced", len(r.forced)),
	)
	for _, info := range r.Manifest() {
		slog.DebugContext(ctx, "initjob task registered",
			slog.String("task_id", info.ID), slog.String("description", info.Description))
	}

	succeeded, err := r.rec.SucceededTaskIDs(ctx)
	if err != nil {
		r.finishRun(ctx, runID, StatusFailed, "query succeeded tasks: "+err.Error())
		return fmt.Errorf("initjob: query succeeded tasks: %w", err)
	}

	var (
		failed   int
		firstErr error
	)

	for _, task := range r.tasks {
		id := task.ID()

		if _, done := succeeded[id]; done {
			if _, force := r.forced[id]; !force {
				slog.InfoContext(ctx, "initjob task already succeeded, skip", r.attrs(id)...)
				continue
			}
			slog.WarnContext(ctx, "initjob task forced to run again although it already succeeded", r.attrs(id)...)
		}

		if safeShouldSkip(ctx, task) {
			// Skipped tasks write no record, so the task row keeps the result
			// of the last run that actually executed it.
			slog.InfoContext(ctx, "initjob task skipped by ShouldSkip", r.attrs(id)...)
			continue
		}

		if err := r.rec.MarkRunning(ctx, runID, id, r.descs[id]); err != nil {
			r.finishRun(ctx, runID, StatusFailed, "mark running: "+err.Error())
			return fmt.Errorf("initjob: mark task %s running: %w", id, err)
		}
		slog.InfoContext(ctx, "initjob task running", r.attrs(id)...)

		taskErr := safeRun(ctx, task)
		if taskErr == nil {
			if err := r.rec.MarkSucceeded(ctx, runID, id); err != nil {
				r.finishRun(ctx, runID, StatusFailed, "mark succeeded: "+err.Error())
				return fmt.Errorf("initjob: mark task %s succeeded: %w", id, err)
			}
			slog.InfoContext(ctx, "initjob task succeeded", r.attrs(id)...)
			continue
		}

		failed++
		if firstErr == nil {
			firstErr = taskErr
		}

		// On SIGTERM the task usually returns ctx.Err(). Leave the row
		// running instead of recording a failure, so the next startup's
		// AbortStale marks it aborted — more accurate than "failed".
		if ctx.Err() != nil {
			slog.WarnContext(ctx, "initjob run canceled while task was running, leaving task record to AbortStale",
				append(r.attrs(id), slog.Any("err", taskErr))...)
			break
		}

		if err := r.rec.MarkFailed(ctx, runID, id, truncate(taskErr.Error(), maxErrorTextLen)); err != nil {
			slog.ErrorContext(ctx, "initjob mark task failed error",
				append(r.attrs(id), slog.Any("err", err))...)
		}
		slog.ErrorContext(ctx, "initjob task failed", append(r.attrs(id), slog.Any("err", taskErr))...)

		if r.opts.Mode == ModeStrict {
			slog.ErrorContext(ctx, "initjob strict mode: aborting remaining tasks", r.attrs(id)...)
			break
		}
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		r.finishRun(ctx, runID, StatusAborted, ctxErr.Error())
		return ctxErr
	}
	if failed > 0 {
		r.finishRun(ctx, runID, StatusFailed, firstErr.Error())
		return firstErr
	}

	r.finishRun(ctx, runID, StatusSucceeded, "")
	slog.InfoContext(ctx, "initjob run succeeded", slog.String("run_id", runID))
	return nil
}

// finishRun writes the final run state. context.WithoutCancel keeps the write
// alive after SIGTERM, otherwise the run would stay running forever.
func (r *Runner) finishRun(ctx context.Context, runID string, status Status, errText string) {
	ctx = context.WithoutCancel(ctx)

	exitCode := 0
	if status != StatusSucceeded {
		exitCode = 1
	}
	if err := r.rec.FinishRun(ctx, runID, status, exitCode, trimText(errText)); err != nil {
		slog.ErrorContext(ctx, "initjob finish run error",
			slog.String("run_id", runID), slog.String("status", string(status)), slog.Any("err", err))
	}
}

// safeRun converts a task panic into a failure: the framework reports failure
// through records and exit codes, never through a panic, but it still contains
// unexpected ones.
func safeRun(ctx context.Context, task Task) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.ErrorContext(ctx, "initjob task panicked",
				slog.String("task_id", task.ID()),
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			err = fmt.Errorf("task %s panicked: %v", task.ID(), rec)
		}
	}()

	return task.Run(ctx)
}

// safeShouldSkip contains a ShouldSkip panic and treats it as "not skipped".
func safeShouldSkip(ctx context.Context, task Task) (skip bool) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.ErrorContext(ctx, "initjob task ShouldSkip panicked, treating as not skipped",
				slog.String("task_id", task.ID()),
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			skip = false
		}
	}()

	return task.ShouldSkip(ctx)
}

// safeDescription reads the task description once at startup; a panic there
// only costs the description.
func safeDescription(task Task) (desc string) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("initjob task Description panicked",
				slog.String("task_id", task.ID()),
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			desc = ""
		}
	}()

	return task.Description()
}

func validTaskID(id string) bool {
	if len(id) != taskIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return false
		}
	}
	return true
}

// truncate cuts on a rune boundary so a multi-byte character is never split.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "...(truncated)"
}

func trimText(s string) string {
	if s == "" {
		return ""
	}
	return truncate(strings.TrimSpace(s), maxErrorTextLen)
}
