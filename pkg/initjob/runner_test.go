package initjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- test doubles ----------

type testTask struct {
	id   string
	desc string
	skip bool
	run  func(ctx context.Context) error
}

func (t *testTask) ID() string { return t.id }

func (t *testTask) Description() string { return t.desc }

func (t *testTask) ShouldSkip(context.Context) bool { return t.skip }

func (t *testTask) Run(ctx context.Context) error {
	if t.run == nil {
		return nil
	}
	return t.run(ctx)
}

// baseOnlyTask implements only ID and Run: proof that embedding Base suffices.
type baseOnlyTask struct {
	Base
	id string
}

func (t *baseOnlyTask) ID() string { return t.id }

func (t *baseOnlyTask) Run(context.Context) error { return nil }

type fakeRun struct {
	mode     Mode
	pod      string
	status   Status
	exitCode int
	errText  string
	finished bool
}

type fakeRecorder struct {
	calls     []string
	succeeded map[string]struct{}
	runs      map[string]*fakeRun
	runOrder  []string
	taskState map[string][]Status
	taskDesc  map[string]string
	taskErr   map[string]string
}

func newFakeRecorder() *fakeRecorder {
	return &fakeRecorder{
		succeeded: map[string]struct{}{},
		runs:      map[string]*fakeRun{},
		taskState: map[string][]Status{},
		taskDesc:  map[string]string{},
		taskErr:   map[string]string{},
	}
}

func (f *fakeRecorder) log(name string) { f.calls = append(f.calls, name) }

func (f *fakeRecorder) AbortStale(context.Context) error {
	f.log("AbortStale")
	return nil
}

func (f *fakeRecorder) StartRun(_ context.Context, mode Mode, pod string) (string, error) {
	f.log("StartRun")
	id := fmt.Sprintf("run-%d", len(f.runOrder)+1)
	f.runs[id] = &fakeRun{mode: mode, pod: pod}
	f.runOrder = append(f.runOrder, id)
	return id, nil
}

func (f *fakeRecorder) FinishRun(
	_ context.Context, runID string, status Status, exitCode int, errText string,
) error {
	f.log("FinishRun")
	run, ok := f.runs[runID]
	if !ok {
		return fmt.Errorf("unknown run %s", runID)
	}
	run.status, run.exitCode, run.errText, run.finished = status, exitCode, errText, true
	return nil
}

func (f *fakeRecorder) SucceededTaskIDs(context.Context) (map[string]struct{}, error) {
	f.log("SucceededTaskIDs")
	return f.succeeded, nil
}

func (f *fakeRecorder) MarkRunning(
	_ context.Context, _ string, taskID string, description string,
) error {
	f.log("MarkRunning:" + taskID)
	f.taskState[taskID] = append(f.taskState[taskID], StatusRunning)
	f.taskDesc[taskID] = description
	return nil
}

func (f *fakeRecorder) MarkSucceeded(_ context.Context, _ string, taskID string) error {
	f.log("MarkSucceeded:" + taskID)
	f.taskState[taskID] = append(f.taskState[taskID], StatusSucceeded)
	return nil
}

func (f *fakeRecorder) MarkFailed(_ context.Context, _ string, taskID string, errText string) error {
	f.log("MarkFailed:" + taskID)
	f.taskState[taskID] = append(f.taskState[taskID], StatusFailed)
	f.taskErr[taskID] = errText
	return nil
}

func (f *fakeRecorder) singleRun(t *testing.T) *fakeRun {
	t.Helper()
	require.Len(t, f.runOrder, 1, "each case must produce exactly one run")
	return f.runs[f.runOrder[0]]
}

// ---------- helpers ----------

// newTasks builds tasks that append their id to exec when run.
func newTasks(exec *[]string, ids ...string) []Task {
	tasks := make([]Task, 0, len(ids))
	for _, id := range ids {
		tasks = append(tasks, &testTask{
			id: id,
			run: func(context.Context) error {
				*exec = append(*exec, id)
				return nil
			},
		})
	}
	return tasks
}

func mustRunner(t *testing.T, tasks []Task, rec Recorder, opts Options) *Runner {
	t.Helper()
	runner, err := NewRunner(tasks, rec, opts)
	require.NoError(t, err)
	return runner
}

// ---------- construction and validation ----------

func TestBase_ProvidesDefaults(t *testing.T) {
	var task Task = &baseOnlyTask{id: "202610050001"}

	assert.False(t, task.ShouldSkip(t.Context()))
	assert.Empty(t, task.Description())
	assert.NoError(t, task.Run(t.Context()))
}

func TestRunner_ManifestKeepsOrderAndDescription(t *testing.T) {
	tasks := []Task{
		&testTask{id: "202610050002", desc: "second"},
		&testTask{id: "202610050001", desc: "first"},
	}
	runner := mustRunner(t, tasks, newFakeRecorder(), Options{})

	assert.Equal(t, []TaskInfo{
		{ID: "202610050001", Description: "first"},
		{ID: "202610050002", Description: "second"},
	}, runner.Manifest())
	assert.Equal(t, []string{"202610050001", "202610050002"}, runner.TaskIDs())
}

func TestNewRunner_DescriptionPanicDoesNotBlockStartup(t *testing.T) {
	runner := mustRunner(t, []Task{&panicDescTask{id: "202610050001"}}, newFakeRecorder(), Options{})

	require.Len(t, runner.Manifest(), 1)
	assert.Empty(t, runner.Manifest()[0].Description)
	require.NoError(t, runner.Run(t.Context()))
}

type panicDescTask struct {
	Base
	id string
}

func (t *panicDescTask) ID() string { return t.id }

func (t *panicDescTask) Description() string { panic("boom in Description") }

func (t *panicDescTask) Run(context.Context) error { return nil }

func TestNewRunner_SortsByIDAscending(t *testing.T) {
	var exec []string
	runner := mustRunner(t,
		newTasks(&exec, "202610050003", "202610050001", "202610050002"),
		newFakeRecorder(), Options{})

	assert.Equal(t, []string{"202610050001", "202610050002", "202610050003"}, runner.TaskIDs())
}

func TestNewRunner_DoesNotMutateCallerSlice(t *testing.T) {
	var exec []string
	tasks := newTasks(&exec, "202610050002", "202610050001")

	_, err := NewRunner(tasks, newFakeRecorder(), Options{})
	require.NoError(t, err)

	assert.Equal(t, "202610050002", tasks[0].ID(), "the caller slice must not be reordered")
}

func TestNewRunner_AcceptsEmptyTaskList(t *testing.T) {
	runner := mustRunner(t, nil, newFakeRecorder(), Options{})
	assert.Empty(t, runner.TaskIDs())
}

func TestNewRunner_RejectsInvalidInput(t *testing.T) {
	var exec []string

	cases := []struct {
		name    string
		tasks   []Task
		rec     Recorder
		opts    Options
		wantErr string
	}{
		{name: "nil recorder", tasks: nil, rec: nil, wantErr: "recorder is nil"},
		{
			name:    "duplicated id",
			tasks:   newTasks(&exec, "202610050001", "202610050001"),
			rec:     newFakeRecorder(),
			wantErr: "duplicated task id",
		},
		{
			name:    "id too short",
			tasks:   newTasks(&exec, "20261005001"),
			rec:     newFakeRecorder(),
			wantErr: "is invalid",
		},
		{
			name:    "id not numeric",
			tasks:   newTasks(&exec, "20261005000a"),
			rec:     newFakeRecorder(),
			wantErr: "is invalid",
		},
		{
			name:    "id empty",
			tasks:   newTasks(&exec, ""),
			rec:     newFakeRecorder(),
			wantErr: "is invalid",
		},
		{
			name:    "nil task",
			tasks:   []Task{nil},
			rec:     newFakeRecorder(),
			wantErr: "is nil",
		},
		{
			name:    "unknown mode",
			tasks:   nil,
			rec:     newFakeRecorder(),
			opts:    Options{Mode: "whatever"},
			wantErr: "unknown mode",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRunner(tc.tasks, tc.rec, tc.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestNewRunner_DefaultModeIsStrict(t *testing.T) {
	rec := newFakeRecorder()
	runner := mustRunner(t, nil, rec, Options{})

	require.NoError(t, runner.Run(t.Context()))
	assert.Equal(t, ModeStrict, rec.singleRun(t).mode)
}

// ---------- skip semantics ----------

func TestRunner_SkipsTasksThatAlreadySucceeded(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()
	rec.succeeded["202610050002"] = struct{}{}
	runner := mustRunner(t,
		newTasks(&exec, "202610050001", "202610050002", "202610050003"), rec, Options{})

	require.NoError(t, runner.Run(t.Context()))

	assert.Equal(t, []string{"202610050001", "202610050003"}, exec)
	assert.NotContains(t, rec.taskState, "202610050002", "a skipped task must not be recorded")
	assert.Equal(t, StatusSucceeded, rec.singleRun(t).status)
}

func TestRunner_ForceRerunsSucceededTask(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()
	rec.succeeded["202610050002"] = struct{}{}
	runner := mustRunner(t,
		newTasks(&exec, "202610050001", "202610050002", "202610050003"), rec,
		Options{ForceTaskIDs: []string{"202610050002"}})

	require.NoError(t, runner.Run(t.Context()))

	assert.Equal(t, []string{"202610050001", "202610050002", "202610050003"}, exec)
	assert.Equal(t, []Status{StatusRunning, StatusSucceeded}, rec.taskState["202610050002"])
}

func TestNewRunner_IgnoresMalformedForceID(t *testing.T) {
	rec := newFakeRecorder()
	runner := mustRunner(t, nil, rec, Options{ForceTaskIDs: []string{"", "  ", "not-a-number"}})

	require.NoError(t, runner.Run(t.Context()))
	assert.Equal(t, StatusSucceeded, rec.singleRun(t).status)
}

func TestRunner_StoresTaskDescriptionInRecord(t *testing.T) {
	rec := newFakeRecorder()
	tasks := []Task{&testTask{id: "202610050001", desc: "seed default models"}}

	require.NoError(t, mustRunner(t, tasks, rec, Options{}).Run(t.Context()))

	assert.Equal(t, "seed default models", rec.taskDesc["202610050001"])
}

func TestRunner_ShouldSkipIsNotRecorded(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()
	tasks := []Task{
		&testTask{id: "202610050001", run: func(context.Context) error {
			exec = append(exec, "202610050001")
			return nil
		}},
		&testTask{id: "202610050002", skip: true, run: func(context.Context) error {
			exec = append(exec, "202610050002")
			return nil
		}},
	}

	require.NoError(t, mustRunner(t, tasks, rec, Options{}).Run(t.Context()))

	assert.Equal(t, []string{"202610050001"}, exec)
	assert.NotContains(t, rec.taskState, "202610050002", "a task skipped by ShouldSkip must not be recorded")
}

func TestRunner_ShouldSkipPanicTreatedAsNotSkipped(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()
	tasks := []Task{&panicSkipTask{id: "202610050001", exec: &exec}}

	require.NoError(t, mustRunner(t, tasks, rec, Options{}).Run(t.Context()))

	assert.Equal(t, []string{"202610050001"}, exec, "a ShouldSkip panic must fall back to running")
	assert.Equal(t, StatusSucceeded, rec.singleRun(t).status)
}

type panicSkipTask struct {
	Base
	id   string
	exec *[]string
}

func (t *panicSkipTask) ID() string { return t.id }

func (t *panicSkipTask) ShouldSkip(context.Context) bool { panic("boom in ShouldSkip") }

func (t *panicSkipTask) Run(context.Context) error {
	*t.exec = append(*t.exec, t.id)
	return nil
}

// ---------- failure policy and exit semantics ----------

func TestRunner_StrictModeAbortsOnFirstFailure(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()
	boom := errors.New("boom")
	tasks := newTasks(&exec, "202610050001", "202610050002", "202610050003")
	tasks[1] = &testTask{id: "202610050002", run: func(context.Context) error {
		exec = append(exec, "202610050002")
		return boom
	}}

	err := mustRunner(t, tasks, rec, Options{Mode: ModeStrict}).Run(t.Context())

	require.ErrorIs(t, err, boom)
	assert.Equal(t, []string{"202610050001", "202610050002"}, exec, "strict mode must abort at once")
	assert.Equal(t, []Status{StatusRunning, StatusFailed}, rec.taskState["202610050002"])
	assert.Equal(t, "boom", rec.taskErr["202610050002"])

	run := rec.singleRun(t)
	assert.Equal(t, StatusFailed, run.status)
	assert.Equal(t, 1, run.exitCode)
	assert.Contains(t, run.errText, "boom")
}

func TestRunner_LooseModeContinuesAndStillFails(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()
	boom := errors.New("boom")
	tasks := newTasks(&exec, "202610050001", "202610050002", "202610050003")
	tasks[1] = &testTask{id: "202610050002", run: func(context.Context) error {
		exec = append(exec, "202610050002")
		return boom
	}}

	err := mustRunner(t, tasks, rec, Options{Mode: ModeLoose}).Run(t.Context())

	require.ErrorIs(t, err, boom)
	assert.Equal(t, []string{"202610050001", "202610050002", "202610050003"}, exec, "loose mode must keep going")

	run := rec.singleRun(t)
	assert.Equal(t, StatusFailed, run.status)
	assert.Equal(t, 1, run.exitCode, "loose mode must still exit non-zero, or k8s never retries")
}

func TestRunner_SuccessPathExitsZero(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()

	require.NoError(t, mustRunner(t,
		newTasks(&exec, "202610050001", "202610050002"), rec, Options{}).Run(t.Context()))

	run := rec.singleRun(t)
	assert.Equal(t, StatusSucceeded, run.status)
	assert.Equal(t, 0, run.exitCode)
	assert.True(t, run.finished)
}

func TestRunner_AbortStaleRunsBeforeStarting(t *testing.T) {
	rec := newFakeRecorder()

	require.NoError(t, mustRunner(t, nil, rec, Options{}).Run(t.Context()))

	require.NotEmpty(t, rec.calls)
	assert.Equal(t, "AbortStale", rec.calls[0], "stale running records must be closed first")
	assert.Equal(t, "StartRun", rec.calls[1])
}

func TestRunner_PassesModeAndPodToRecorder(t *testing.T) {
	rec := newFakeRecorder()

	require.NoError(t, mustRunner(t, nil, rec, Options{Mode: ModeLoose, Pod: "initjob-abc"}).Run(t.Context()))

	run := rec.singleRun(t)
	assert.Equal(t, ModeLoose, run.mode)
	assert.Equal(t, "initjob-abc", run.pod)
}

func TestRunner_PanicRecordedAsFailure(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()
	tasks := newTasks(&exec, "202610050001", "202610050002")
	tasks[1] = &testTask{id: "202610050002", run: func(context.Context) error {
		panic("kaboom")
	}}

	err := mustRunner(t, tasks, rec, Options{Mode: ModeStrict}).Run(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "panicked")
	assert.Contains(t, rec.taskErr["202610050002"], "panicked")

	run := rec.singleRun(t)
	assert.Equal(t, StatusFailed, run.status, "a panic must be recovered into a failure, not a crash")
	assert.Equal(t, 1, run.exitCode)
}

func TestRunner_CanceledContextAbortsRun(t *testing.T) {
	var exec []string
	rec := newFakeRecorder()

	ctx, cancel := context.WithCancel(t.Context())
	tasks := []Task{
		&testTask{id: "202610050001", run: func(c context.Context) error {
			exec = append(exec, "202610050001")
			cancel()
			return c.Err()
		}},
		&testTask{id: "202610050002", run: func(context.Context) error {
			exec = append(exec, "202610050002")
			return nil
		}},
	}

	err := mustRunner(t, tasks, rec, Options{}).Run(ctx)

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{"202610050001"}, exec, "no further task after cancellation")

	run := rec.singleRun(t)
	assert.Equal(t, StatusAborted, run.status)
	assert.Equal(t, 1, run.exitCode)

	// An interrupted task is not "failed": leave it running so the next
	// AbortStale marks it aborted.
	assert.Equal(t, []Status{StatusRunning}, rec.taskState["202610050001"])
	assert.NotContains(t, rec.taskState, "202610050002")
}

// ---------- internal helpers ----------

func TestTruncate_KeepsRunesIntact(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 10))
	assert.Equal(t, "abc...(truncated)", truncate("abcdef", 3))
	// A cut landing inside a multi-byte rune must fall back to the rune boundary.
	got := truncate("中文字符", 4)
	assert.True(t, strings.HasPrefix(got, "中"))
	assert.NotContains(t, got, "\ufffd")
}
