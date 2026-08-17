package service

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sandboxerrors "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/errors"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/stretchr/testify/require"
)

type stubSandbox struct {
	id  string
	key entity.SandboxKey
}

func (s *stubSandbox) Id() string { return s.id }
func (s *stubSandbox) Description() entity.SandboxDescription {
	return entity.SandboxDescription{Id: s.id, Key: s.key, Runtime: "test"}
}

func (s *stubSandbox) Run(context.Context, entity.Command) (entity.Execution, error) {
	return entity.Execution{}, nil
}
func (s *stubSandbox) WriteFile(context.Context, string, io.Reader) error { return nil }
func (s *stubSandbox) ReadFile(context.Context, string, ...entity.SandboxOption) ([]byte, error) {
	return nil, nil
}
func (s *stubSandbox) ReadFile2(context.Context, string, ...entity.SandboxOption) (io.ReadCloser, error) {
	return nil, nil
}

func (s *stubSandbox) ListDir(context.Context, string) ([]entity.ListDirItem, error) {
	return nil, nil
}
func (s *stubSandbox) EditFile(context.Context, string, string, string) error { return nil }

type fakeRepo struct {
	mu   sync.Mutex
	desc map[string]entity.SandboxDescription
	ttls map[string]time.Duration
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		desc: make(map[string]entity.SandboxDescription),
		ttls: make(map[string]time.Duration),
	}
}

func (r *fakeRepo) cacheKey(key entity.SandboxKey) string {
	return key.UserId + ":" + key.NotebookId.String()
}

func (r *fakeRepo) GetSandbox(_ context.Context, key entity.SandboxKey) (entity.SandboxDescription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.desc[r.cacheKey(key)]
	if !ok {
		return entity.SandboxDescription{}, sandboxerrors.ErrSandboxNotFound
	}
	return d, nil
}

func (r *fakeRepo) DeleteSandbox(_ context.Context, key entity.SandboxKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.desc, r.cacheKey(key))
	delete(r.ttls, r.cacheKey(key))
	return nil
}

func (r *fakeRepo) SetSandbox(_ context.Context, key entity.SandboxKey, desc entity.SandboxDescription, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.desc[r.cacheKey(key)] = desc
	r.ttls[r.cacheKey(key)] = ttl
	return nil
}

type fakeMgr struct {
	mu          sync.Mutex
	createSpecs []entity.Spec
	createCount int
	sandboxes   map[string]entity.Sandbox
	getCalls    []string
}

func newFakeMgr() *fakeMgr {
	return &fakeMgr{sandboxes: make(map[string]entity.Sandbox)}
}

func (m *fakeMgr) CreateSandbox(_ context.Context, key entity.SandboxKey, spec entity.Spec) (entity.Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCount++
	m.createSpecs = append(m.createSpecs, spec)
	id := "sb-" + key.NotebookId.String()
	sb := &stubSandbox{id: id, key: key}
	m.sandboxes[id] = sb
	return sb, nil
}

func (m *fakeMgr) GetSandbox(_ context.Context, sandboxId string) (entity.Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls = append(m.getCalls, sandboxId)
	sb, ok := m.sandboxes[sandboxId]
	if !ok {
		return nil, pkgerr.ErrNoRecord.Msg("sandbox gone")
	}
	return sb, nil
}

func (m *fakeMgr) DeleteSandbox(_ context.Context, sandboxId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sandboxes, sandboxId)
	return nil
}

type recordingLock struct {
	mu       sync.Mutex
	locked   []string
	unlocked []string
}

func (l *recordingLock) Lock(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.locked = append(l.locked, key)
	return nil
}

func (l *recordingLock) Unlock(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.unlocked = append(l.unlocked, key)
	return nil
}

func (l *recordingLock) Check(context.Context, string) (bool, error) { return false, nil }

func testKey(t *testing.T) entity.SandboxKey {
	t.Helper()
	nb, err := valobj.NewIdFromString("01900000-0000-7000-8000-000000000001")
	require.NoError(t, err)
	return entity.SandboxKey{UserId: "u1", NotebookId: nb}
}

func TestGetOrCreateSandbox_DefaultTTLAligned(t *testing.T) {
	repo := newFakeRepo()
	mgr := newFakeMgr()
	lock := &recordingLock{}
	svc := New(repo, mgr, lock)

	sb, err := svc.GetOrCreateSandbox(context.Background(), testKey(t), entity.Spec{})
	require.NoError(t, err)
	require.NotNil(t, sb)

	require.Equal(t, 1, mgr.createCount)
	require.Equal(t, 60*time.Minute, mgr.createSpecs[0].TTL)

	key := testKey(t)
	require.Equal(t, 60*time.Minute, repo.ttls[repo.cacheKey(key)])
}

func TestGetOrCreateSandbox_ExplicitTTLPropagated(t *testing.T) {
	repo := newFakeRepo()
	mgr := newFakeMgr()
	svc := New(repo, mgr, &recordingLock{})

	ttl := 3 * time.Minute
	_, err := svc.GetOrCreateSandbox(context.Background(), testKey(t), entity.Spec{TTL: ttl})
	require.NoError(t, err)

	require.Equal(t, ttl, mgr.createSpecs[0].TTL)
	require.Equal(t, ttl, repo.ttls[repo.cacheKey(testKey(t))])
}

func TestGetOrCreateSandbox_UsesDistributedLock(t *testing.T) {
	repo := newFakeRepo()
	mgr := newFakeMgr()
	lock := &recordingLock{}
	svc := New(repo, mgr, lock)

	key := testKey(t)
	_, err := svc.GetOrCreateSandbox(context.Background(), key, entity.Spec{})
	require.NoError(t, err)

	want := sandboxLockKey(key)
	require.Equal(t, []string{want}, lock.locked)
	require.Equal(t, []string{want}, lock.unlocked)
}

func TestGetOrCreateSandbox_DoubleCheckReusesExisting(t *testing.T) {
	key := testKey(t)
	repo := newFakeRepo()
	mgr := newFakeMgr()

	existing := &stubSandbox{id: "sb-existing", key: key}
	mgr.sandboxes[existing.id] = existing
	require.NoError(t, repo.SetSandbox(context.Background(), key, existing.Description(), time.Minute))

	svc := New(repo, mgr, &recordingLock{})
	sb, err := svc.GetOrCreateSandbox(context.Background(), key, entity.Spec{})
	require.NoError(t, err)
	require.Equal(t, "sb-existing", sb.Id())
	require.Equal(t, 0, mgr.createCount)
}

func TestGetOrCreateSandbox_StaleCacheClearedOnce(t *testing.T) {
	key := testKey(t)
	repo := newFakeRepo()
	mgr := newFakeMgr()

	// Redis 有绑定，但远端沙箱已不存在
	require.NoError(t, repo.SetSandbox(context.Background(), key, entity.SandboxDescription{
		Id:  "sb-dead",
		Key: key,
	}, time.Hour))

	svc := New(repo, mgr, &recordingLock{})
	sb, err := svc.GetOrCreateSandbox(context.Background(), key, entity.Spec{})
	require.NoError(t, err)
	require.Equal(t, 1, mgr.createCount)
	require.NotEqual(t, "sb-dead", sb.Id())

	// 快路径发现失效后应删缓存，锁内不应再对死 id Get 一次
	require.Equal(t, []string{"sb-dead"}, mgr.getCalls)
}
