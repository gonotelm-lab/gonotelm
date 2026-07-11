package syncer

import (
	"context"
	"sync"
	"testing"
	"time"

	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type syncTestRepo struct {
	mu   sync.Mutex
	byId map[valobj.Id]*artifactentity.Artifact
}

func newSyncTestRepo(artifacts ...*artifactentity.Artifact) *syncTestRepo {
	r := &syncTestRepo{byId: make(map[valobj.Id]*artifactentity.Artifact)}
	for _, a := range artifacts {
		r.byId[a.Id] = a
	}
	return r
}

func (s *syncTestRepo) Save(ctx context.Context, a *artifactentity.Artifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byId[a.Id] = a
	return nil
}

func (s *syncTestRepo) FindById(ctx context.Context, id valobj.Id) (*artifactentity.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.byId[id]
	if !ok {
		return nil, nil
	}
	copy := *a
	return &copy, nil
}

func (s *syncTestRepo) ListByNotebookId(ctx context.Context, notebookId valobj.Id, spec *artifactrepo.ListSpec) ([]*artifactentity.Artifact, error) {
	return nil, nil
}

func (s *syncTestRepo) ListByStatus(ctx context.Context, spec *artifactrepo.ListByStatusSpec) ([]*artifactentity.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	statusSet := make(map[artifactentity.Status]bool)
	for _, st := range spec.Statuses {
		statusSet[st] = true
	}
	var result []*artifactentity.Artifact
	for _, a := range s.byId {
		if statusSet[a.Status] {
			copy := *a
			result = append(result, &copy)
			if len(result) >= spec.Limit {
				break
			}
		}
	}
	return result, nil
}

func (s *syncTestRepo) DeleteById(ctx context.Context, id valobj.Id) error { return nil }
func (s *syncTestRepo) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return nil
}

var _ artifactrepo.Repository = &syncTestRepo{}

type stubEventBus struct{}

func (s *stubEventBus) Publish(ctx context.Context, evt event.Event) error { return nil }
func (s *stubEventBus) Subscribe(ctx context.Context, topic, groupID string, handler eventbus.EventBusMessageHandler) error {
	return nil
}
func (s *stubEventBus) Close(ctx context.Context) error { return nil }

var _ eventbus.EventBus = &stubEventBus{}

type syncTestFlow struct {
	mu     sync.Mutex
	states []flowschema.TaskState
	result []byte
	index  int
}

func newSyncTestFlow(states []flowschema.TaskState, result []byte) *syncTestFlow {
	return &syncTestFlow{states: states, result: result}
}

func (f *syncTestFlow) Submit(ctx context.Context, taskType string, payload []byte) (string, error) {
	return "flow-1", nil
}

func (f *syncTestFlow) Get(ctx context.Context, flowTaskId string) (*flow.TaskInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.index >= len(f.states) {
		return &flow.TaskInfo{ID: flowTaskId, State: f.states[len(f.states)-1], Result: f.result}, nil
	}
	state := f.states[f.index]
	f.index++
	return &flow.TaskInfo{ID: flowTaskId, State: state, Result: f.result}, nil
}

func (f *syncTestFlow) Cancel(ctx context.Context, flowTaskId string) error { return nil }
func (f *syncTestFlow) Close() error                                        { return nil }

var _ flow.TaskClient = &syncTestFlow{}

func makeSyncArtifact(status artifactentity.Status, flowTaskId string) *artifactentity.Artifact {
	a, err := artifactentity.NewArtifact(uuid.NewV7(), "u1", artifactentity.KindMindmap, &artifactentity.MindmapPayload{NotebookId: uuid.NewV7()})
	if err != nil {
		panic(err)
	}
	a.Status = status
	a.FlowTaskId = flowTaskId
	return a
}

func TestSyncer_PollOne_ReachesTerminalAndStops(t *testing.T) {
	a := makeSyncArtifact(artifactentity.StatusPending, "ft-1")
	repo := newSyncTestRepo(a)
	flowc := newSyncTestFlow([]flowschema.TaskState{flowschema.TaskState_RUNNING, flowschema.TaskState_DONE}, []byte(`{"key":"val"}`))

	s := NewSyncer(repo, flowc, Config{
		PerTaskInterval: 10 * time.Millisecond,
		GlobalInterval:  10 * time.Millisecond,
		GlobalBatchSize: 100,
	}, &stubEventBus{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.PollOne(ctx, a.Id)

	require.Eventually(t, func() bool {
		got, _ := repo.FindById(ctx, a.Id)
		return got.Status.Completed()
	}, 2*time.Second, 20*time.Millisecond)

	got, _ := repo.FindById(ctx, a.Id)
	assert.True(t, got.Status.Completed())
	assert.Equal(t, []byte(`{"key":"val"}`), got.Result)
}

func TestSyncer_GlobalScan_CatchesPendingArtifacts(t *testing.T) {
	a1 := makeSyncArtifact(artifactentity.StatusPending, "ft-a")
	a2 := makeSyncArtifact(artifactentity.StatusPending, "ft-b")
	repo := newSyncTestRepo(a1, a2)
	flowc := newSyncTestFlow([]flowschema.TaskState{flowschema.TaskState_DONE}, []byte(`result`))

	s := NewSyncer(repo, flowc, Config{
		PerTaskInterval: 10 * time.Millisecond,
		GlobalInterval:  10 * time.Millisecond,
		GlobalBatchSize: 100,
	}, &stubEventBus{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.scanOnce(ctx)

	got1, _ := repo.FindById(ctx, a1.Id)
	assert.True(t, got1.Status.Completed())
	got2, _ := repo.FindById(ctx, a2.Id)
	assert.True(t, got2.Status.Completed())
}

func TestSyncer_Shutdown_StopsLoops(t *testing.T) {
	a := makeSyncArtifact(artifactentity.StatusPending, "ft-s")
	repo := newSyncTestRepo(a)
	flowc := newSyncTestFlow([]flowschema.TaskState{flowschema.TaskState_RUNNING}, nil)

	s := NewSyncer(repo, flowc, Config{
		PerTaskInterval: 10 * time.Millisecond,
		GlobalInterval:  10 * time.Millisecond,
		GlobalBatchSize: 100,
	}, &stubEventBus{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	start := time.Now()
	s.Shutdown(shutdownCtx)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 500*time.Millisecond, "shutdown should complete quickly")
}
