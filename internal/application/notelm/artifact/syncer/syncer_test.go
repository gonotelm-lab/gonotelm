package syncer

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/contract"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
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

func (s *syncTestRepo) Create(ctx context.Context, a *artifactentity.Artifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byId[a.Id] = a
	return nil
}

func (s *syncTestRepo) UpdateExisting(ctx context.Context, a *artifactentity.Artifact) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byId[a.Id]; !ok {
		return 0, nil
	}
	s.byId[a.Id] = a
	return 1, nil
}

func (s *syncTestRepo) FindById(ctx context.Context, id valobj.Id) (*artifactentity.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.byId[id]
	if !ok {
		return nil, artifacterrors.ErrArtifactNotFound
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

var _ eventbus.Publisher = &stubEventBus{}

type syncTestStorage struct {
	mu          sync.Mutex
	deletedKeys []valobj.StoreKey
	calls       int
}

func (s *syncTestStorage) GetObject(context.Context, valobj.StoreKey) ([]byte, *adapter.ObjectInfo, error) {
	return nil, nil, nil
}
func (s *syncTestStorage) GetPartialObject(context.Context, valobj.StoreKey, int64, int64) ([]byte, *adapter.ObjectInfo, error) {
	return nil, nil, nil
}
func (s *syncTestStorage) Upload(context.Context, valobj.StoreKey, []byte, string) error { return nil }
func (s *syncTestStorage) UploadReader(context.Context, valobj.StoreKey, string, io.Reader) error {
	return nil
}
func (s *syncTestStorage) PresignUpload(context.Context, valobj.StoreKey, *adapter.UploadOptions) (*adapter.PresignUploadResult, error) {
	return nil, nil
}
func (s *syncTestStorage) DeleteObject(context.Context, valobj.StoreKey) error { return nil }
func (s *syncTestStorage) BatchDeleteObject(_ context.Context, keys []valobj.StoreKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.deletedKeys = append(s.deletedKeys, keys...)

	return nil
}
func (s *syncTestStorage) PublicURL(context.Context, valobj.StoreKey) (string, error) { return "", nil }
func (s *syncTestStorage) PresignGet(context.Context, valobj.StoreKey) (string, error) {
	return "", nil
}
func (s *syncTestStorage) CheckExist(context.Context, valobj.StoreKey) (bool, error) {
	return false, nil
}

var _ adapter.ObjectStore = &syncTestStorage{}

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
	a, err := artifactentity.NewArtifact(uuid.NewV7(), valobj.NewUid(), artifactentity.KindMindmap, &artifactentity.MindmapPayload{NotebookId: uuid.NewV7()})
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

	// The flow task result is a marshaled contract.WorkerOutput.
	innerResult := []byte("mindmap content")
	out := contract.WorkerOutput{
		Title:      "test title",
		Result:     innerResult,
		ResultKind: "inline",
	}
	workerOutputBytes, err := sonic.Marshal(out)
	require.NoError(t, err)
	flowc := newSyncTestFlow([]flowschema.TaskState{flowschema.TaskState_RUNNING, flowschema.TaskState_DONE}, workerOutputBytes)

	s := New(repo, flowc, &syncTestStorage{}, Config{
		PerTaskInterval: 10 * time.Millisecond,
		GlobalInterval:  10 * time.Millisecond,
		GlobalBatchSize: 100,
	}, &stubEventBus{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.PollOne(ctx, a.Id, a.FlowTaskId)

	require.Eventually(t, func() bool {
		got, _ := repo.FindById(ctx, a.Id)
		return got.Status.Completed()
	}, 2*time.Second, 20*time.Millisecond)

	got, _ := repo.FindById(ctx, a.Id)
	assert.True(t, got.Status.Completed())
	// Result should be the inner content, not the whole WorkerOutput JSON.
	assert.Equal(t, innerResult, got.Result)
	assert.Equal(t, "test title", got.Title)
	assert.Equal(t, artifactentity.ResultKindInline, got.ResultKind)
}

func TestSyncer_GlobalScan_CatchesPendingArtifacts(t *testing.T) {
	out := contract.WorkerOutput{
		Title:      "scanned title",
		Result:     []byte("scanned result"),
		ResultKind: "inline",
	}
	outBytes, err := sonic.Marshal(out)
	require.NoError(t, err)

	a1 := makeSyncArtifact(artifactentity.StatusPending, "ft-a")
	a2 := makeSyncArtifact(artifactentity.StatusPending, "ft-b")
	repo := newSyncTestRepo(a1, a2)
	flowc := newSyncTestFlow([]flowschema.TaskState{flowschema.TaskState_DONE}, outBytes)

	s := New(repo, flowc, &syncTestStorage{}, Config{
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

	s := New(repo, flowc, &syncTestStorage{}, Config{
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

func makeStorageWorkerOutput(t *testing.T, key valobj.StoreKey) []byte {
	t.Helper()

	storageResult, err := sonic.Marshal(contract.StorageResult{StoreKey: key, ContentType: "application/pdf"})
	require.NoError(t, err)

	out, err := sonic.Marshal(contract.WorkerOutput{
		Title:      "slides",
		Result:     storageResult,
		ResultKind: string(artifactentity.ResultKindStorage),
	})
	require.NoError(t, err)

	return out
}

func newOrphanSyncer(t *testing.T, states []flowschema.TaskState, result []byte) (*Syncer, *syncTestStorage) {
	t.Helper()

	storage := &syncTestStorage{}
	flowc := newSyncTestFlow(states, result)
	s := New(newSyncTestRepo(), flowc, storage, Config{
		PerTaskInterval: 10 * time.Millisecond,
		GlobalInterval:  10 * time.Millisecond,
		GlobalBatchSize: 100,
	}, &stubEventBus{})

	return s, storage
}

func TestSyncer_ReapOrphanDeletesStorageObjectOnDone(t *testing.T) {
	key, err := valobj.NewStoreKey("gonotelm-private", "artifact/nb/art.pptx", false)
	require.NoError(t, err)

	s, storage := newOrphanSyncer(t, []flowschema.TaskState{flowschema.TaskState_DONE}, makeStorageWorkerOutput(t, key))

	done, err := s.pollOnce(t.Context(), valobj.NewId(), "ft-orphan")
	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, []valobj.StoreKey{key}, storage.deletedKeys)
}

func TestSyncer_ReapOrphanKeepsPollingWhileTaskRuns(t *testing.T) {
	key, err := valobj.NewStoreKey("gonotelm-private", "artifact/nb/art.pptx", false)
	require.NoError(t, err)

	s, storage := newOrphanSyncer(t, []flowschema.TaskState{flowschema.TaskState_RUNNING, flowschema.TaskState_DONE}, makeStorageWorkerOutput(t, key))
	artifactId := valobj.NewId()

	done, err := s.pollOnce(t.Context(), artifactId, "ft-orphan")
	require.NoError(t, err)
	assert.False(t, done)
	assert.Empty(t, storage.deletedKeys)

	done, err = s.pollOnce(t.Context(), artifactId, "ft-orphan")
	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, []valobj.StoreKey{key}, storage.deletedKeys)
}

func TestSyncer_ReapOrphanDeletesStorageObjectOnCancelled(t *testing.T) {
	key, err := valobj.NewStoreKey("gonotelm-private", "artifact/nb/art.pptx", false)
	require.NoError(t, err)

	s, storage := newOrphanSyncer(t, []flowschema.TaskState{flowschema.TaskState_CANCELLED}, makeStorageWorkerOutput(t, key))

	done, err := s.pollOnce(t.Context(), valobj.NewId(), "ft-orphan")
	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, []valobj.StoreKey{key}, storage.deletedKeys)
}

func TestSyncer_ReapOrphanSkipsInlineResult(t *testing.T) {
	out, err := sonic.Marshal(contract.WorkerOutput{
		Title:      "note",
		Result:     []byte("inline content"),
		ResultKind: string(artifactentity.ResultKindInline),
	})
	require.NoError(t, err)

	s, storage := newOrphanSyncer(t, []flowschema.TaskState{flowschema.TaskState_DONE}, out)

	done, err := s.pollOnce(t.Context(), valobj.NewId(), "ft-orphan")
	require.NoError(t, err)
	assert.True(t, done)
	assert.Zero(t, storage.calls)
}

func TestSyncer_ReapOrphanWithoutFlowTaskId(t *testing.T) {
	// flowTaskId 为空就不该碰 flow：newSyncTestFlow 空状态列表一旦被 Get 会 panic
	s, storage := newOrphanSyncer(t, nil, nil)

	done, err := s.pollOnce(t.Context(), valobj.NewId(), "")
	require.NoError(t, err)
	assert.True(t, done)
	assert.Zero(t, storage.calls)
}
