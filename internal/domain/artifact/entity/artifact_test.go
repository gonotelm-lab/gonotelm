package entity

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactevent "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/event"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewArtifact(t *testing.T) {
	notebookId := uuid.NewV7()
	userId := valobj.NewUid()
	payload := &MindmapPayload{NotebookId: notebookId, SourceIds: []valobj.Id{uuid.NewV7()}}

	got, err := NewArtifact(notebookId, userId, KindMindmap, payload)
	require.NoError(t, err)

	assert.NotEqual(t, valobj.Id{}, got.Id)
	assert.Equal(t, notebookId, got.NotebookId)
	assert.Equal(t, userId, got.UserId)
	assert.Equal(t, KindMindmap, got.Kind)
	assert.Equal(t, StatusPending, got.Status)
	assert.Equal(t, payload, got.Payload)
	assert.False(t, got.IsTerminal())
}

func TestNewArtifact_Validation_Errors(t *testing.T) {
	notebookId := uuid.NewV7()
	validPayload := &MindmapPayload{NotebookId: notebookId}

	tests := []struct {
		name       string
		notebookId valobj.Id
		userId     valobj.Uid
		kind       Kind
		payload    Payload
		errTarget  error
	}{
		{"empty notebook id", valobj.Id{}, valobj.NewUid(), KindMindmap, validPayload, artifacterrors.ErrInvalidNotebookId},
		{"empty user id", notebookId, valobj.Uid{}, KindMindmap, validPayload, artifacterrors.ErrInvalidUserId},
		{"unsupported kind", notebookId, valobj.NewUid(), Kind("bogus"), validPayload, artifacterrors.ErrInvalidKind},
		{"nil payload", notebookId, valobj.NewUid(), KindMindmap, nil, artifacterrors.ErrInvalidPayload},
		{"payload kind mismatch", notebookId, valobj.NewUid(), KindMindmap, &ReportPayload{NotebookId: notebookId}, artifacterrors.ErrPayloadKindMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewArtifact(tt.notebookId, tt.userId, tt.kind, tt.payload)
			assert.ErrorIs(t, err, tt.errTarget)
		})
	}
}

func TestNewArtifact_SetsUpdateTime(t *testing.T) {
	a, err := NewArtifact(uuid.NewV7(), valobj.NewUid(), KindMindmap, &MindmapPayload{NotebookId: uuid.NewV7()})
	require.NoError(t, err)
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactBindFlowTaskId(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("flow-task-1")
	assert.Equal(t, "flow-task-1", a.FlowTaskId)
}

func TestArtifactMarkCompleted(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCompleted([]byte("result"), ResultKindInline, "title")
	assert.Equal(t, StatusCompleted, a.Status)
	assert.Equal(t, []byte("result"), a.Result)
	assert.Equal(t, ResultKindInline, a.ResultKind)
	assert.Equal(t, "title", a.Title)
	assert.True(t, a.IsTerminal())
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactMarkFailed(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	assert.Equal(t, StatusFailed, a.Status)
	assert.True(t, a.IsTerminal())
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactMarkCancelled(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCancelled()
	assert.Equal(t, StatusCancelled, a.Status)
	assert.True(t, a.IsTerminal())
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactMarkRetrying(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCompleted([]byte("old"), ResultKindInline, "old-title")
	a.MarkRetrying("flow-task-2")
	assert.Equal(t, StatusPending, a.Status)
	assert.Equal(t, "flow-task-2", a.FlowTaskId)
	assert.Empty(t, a.Title)
	assert.Empty(t, a.Result)
	assert.Empty(t, a.ResultKind)
	assert.NotZero(t, a.UpdateTime.Value())
}

func TestArtifactCancel_HappyPath(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("ft-1")
	require.NoError(t, a.Cancel())
	assert.Equal(t, StatusCancelled, a.Status)
}

func TestArtifactCancel_TerminalRejected(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("ft-1")
	a.MarkCompleted([]byte("r"), ResultKindInline, "t")
	err := a.Cancel()
	assert.ErrorIs(t, err, artifacterrors.ErrCannotCancelInState)
}

func TestArtifactCancel_NoFlowTaskId(t *testing.T) {
	a := newTestArtifact(t)
	err := a.Cancel()
	assert.ErrorIs(t, err, artifacterrors.ErrInvalidFlowTaskId)
}

func TestArtifactRetry_HappyPath(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	require.NoError(t, a.Retry("ft-2"))
	assert.Equal(t, StatusPending, a.Status)
	assert.Equal(t, "ft-2", a.FlowTaskId)
}

func TestArtifactRetry_NotFailedOrCancelled(t *testing.T) {
	a := newTestArtifact(t)
	err := a.Retry("ft-2")
	assert.ErrorIs(t, err, artifacterrors.ErrCannotRetryInState)
}

func TestArtifactIsOwner(t *testing.T) {
	a := newTestArtifact(t)
	assert.True(t, a.IsOwner(a.UserId))
	assert.False(t, a.IsOwner(valobj.NewUid()))
}

func TestArtifactMarkCompleted_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCompleted([]byte("r"), ResultKindInline, "title")
	events := a.PullEvents()
	require.Len(t, events, 1)
	assert.Equal(t, artifactevent.TopicArtifactEvent, events[0].Topic())
}

func TestArtifactMarkFailed_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	events := a.PullEvents()
	require.Len(t, events, 1)
}

func TestArtifactMarkCancelled_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkCancelled()
	events := a.PullEvents()
	require.Len(t, events, 1)
}

func TestArtifactCancel_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.BindFlowTaskId("ft-1")
	require.NoError(t, a.Cancel())
	events := a.PullEvents()
	require.Len(t, events, 1)
}

func TestArtifactRetry_EmitsEvent(t *testing.T) {
	a := newTestArtifact(t)
	a.MarkFailed()
	a.PullEvents()
	require.NoError(t, a.Retry("ft-2"))
	events := a.PullEvents()
	require.Len(t, events, 1)
}

func TestArtifactUpdateTitle(t *testing.T) {
	t.Run("completed trims and allows empty", func(t *testing.T) {
		a := newTestArtifact(t)
		a.MarkCompleted([]byte("r"), ResultKindInline, "old")
		require.NoError(t, a.UpdateTitle("  hello  "))
		assert.Equal(t, "hello", a.Title)
		require.NoError(t, a.UpdateTitle("   "))
		assert.Equal(t, "", a.Title)
	})

	t.Run("truncates to MaxTitleLength runes", func(t *testing.T) {
		a := newTestArtifact(t)
		a.MarkCompleted([]byte("r"), ResultKindInline, "old")
		long := strings.Repeat("中", MaxTitleLength+5)
		require.NoError(t, a.UpdateTitle(long))
		assert.Equal(t, MaxTitleLength, utf8.RuneCountInString(a.Title))
	})

	t.Run("rejects non-completed", func(t *testing.T) {
		a := newTestArtifact(t)
		err := a.UpdateTitle("x")
		assert.ErrorIs(t, err, artifacterrors.ErrCannotUpdateTitleInState)
	})
}

func newTestArtifact(t *testing.T) *Artifact {
	t.Helper()
	a, err := NewArtifact(uuid.NewV7(), valobj.NewUid(), KindMindmap, &MindmapPayload{NotebookId: uuid.NewV7()})
	require.NoError(t, err)
	return a
}

func newTestStoreKey(t *testing.T) valobj.StoreKey {
	t.Helper()
	key, err := valobj.NewStoreKey("gonotelm-private", "artifact/notebook/result.pptx", false)
	require.NoError(t, err)

	return key
}

func TestParseStorageResult(t *testing.T) {
	key := newTestStoreKey(t)

	t.Run("round-trips the storage envelope", func(t *testing.T) {
		data, err := sonic.Marshal(StorageResult{StoreKey: key, ContentType: "application/pdf"})
		require.NoError(t, err)

		sr, ok := ParseStorageResult(data)
		require.True(t, ok)
		assert.Equal(t, key, sr.StoreKey)
		assert.Equal(t, "application/pdf", sr.ContentType)
	})

	t.Run("rejects broken json", func(t *testing.T) {
		_, ok := ParseStorageResult([]byte(`{"store_key":`))
		assert.False(t, ok)
	})

	t.Run("rejects an invalid store key", func(t *testing.T) {
		_, ok := ParseStorageResult([]byte(`{"store_key":{"bucket":"","key":""}}`))
		assert.False(t, ok)
	})
}

func TestArtifactObjectStoreKeys(t *testing.T) {
	t.Run("storage result yields the object key", func(t *testing.T) {
		a := newTestArtifact(t)
		key := newTestStoreKey(t)
		result, err := sonic.Marshal(StorageResult{StoreKey: key, ContentType: "application/pdf"})
		require.NoError(t, err)

		a.MarkCompleted(result, ResultKindStorage, "slides")

		keys := a.ObjectStoreKeys()
		require.Len(t, keys, 1)
		assert.Equal(t, key, keys[0])
	})

	t.Run("inline result has no object key", func(t *testing.T) {
		a := newTestArtifact(t)
		a.MarkCompleted([]byte(`{"markdown":"# hi"}`), ResultKindInline, "mindmap")

		assert.Empty(t, a.ObjectStoreKeys())
	})

	t.Run("empty result has no object key", func(t *testing.T) {
		a := newTestArtifact(t)
		a.ResultKind = ResultKindStorage

		assert.Empty(t, a.ObjectStoreKeys())
	})

	t.Run("broken json has no object key", func(t *testing.T) {
		a := newTestArtifact(t)
		a.MarkCompleted([]byte(`{"store_key":`), ResultKindStorage, "slides")

		assert.Empty(t, a.ObjectStoreKeys())
	})

	t.Run("invalid store key is dropped", func(t *testing.T) {
		a := newTestArtifact(t)
		a.MarkCompleted([]byte(`{"store_key":{"bucket":"","key":""}}`), ResultKindStorage, "slides")

		assert.Empty(t, a.ObjectStoreKeys())
	})

	t.Run("nil artifact has no object key", func(t *testing.T) {
		var a *Artifact

		assert.Empty(t, a.ObjectStoreKeys())
	})
}
