package clickhouse

import (
	"context"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	pkgch "github.com/gonotelm-lab/gonotelm/pkg/clickhouse"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStore(t *testing.T) (*LLMLogStoreImpl, *pkgch.Conn) {
	t.Helper()
	driver, err := openTestConn("default")
	require.NoError(t, err)
	conn := pkgch.NewConn(driver)
	t.Cleanup(func() { _ = conn.Close() })
	store, err := NewLLMLogStoreImpl(context.Background(), conn)
	require.NoError(t, err)
	return store, conn
}

func queryLLMLog(t *testing.T, id string) (schema.LLMLog, bool) {
	t.Helper()
	var rows []schema.LLMLog
	err := testDriver.Select(context.Background(), &rows,
		"SELECT * FROM "+testTable+" WHERE id = ?", id)
	require.NoError(t, err)
	if len(rows) == 0 {
		return schema.LLMLog{}, false
	}
	return rows[0], true
}

func fullLLMLog() *schema.LLMLog {
	modelParams := `{"temperature":0.7}`
	input := "hello"
	output := "world"
	totalCost := decimal.RequireFromString("0.001234")
	errMsg := "timeout"
	now := time.Now().UTC().Truncate(time.Millisecond)
	start := now.Add(-2 * time.Second)
	finish := now.Add(-time.Second)

	log := &schema.LLMLog{
		ID:              "full-field-id",
		GroupID:         "group-full",
		TraceID:         "trace-full",
		UserID:          "user-full",
		Scene:           "artifact",
		Model:           "gpt-4o",
		ModelProvider:   "openai",
		ModelParameters: &modelParams,
		CallStartTime:   start,
		CallFinishTime:  finish,
		Input:           &input,
		Output:          &output,
		ToolCalls: []schema.LLMLogToolCall{
			{
				Name:      "hello",
				Arguments: "ok",
			},
			{
				Name:      "hi",
				Arguments: "yes",
			},
		},
		ToolDefinitions: map[string]string{
			"search": "search the web",
		},
		UsageDetails: map[string]uint64{
			"prompt_tokens":     100,
			"completion_tokens": 50,
		},
		CostDetails: map[string]decimal.Decimal{
			"prompt_cost":     decimal.RequireFromString("0.000500"),
			"completion_cost": decimal.RequireFromString("0.000734"),
		},
		TotalCost:  &totalCost,
		CreateTime: now,
		UpdateTime: now,
		Metadata: map[string]string{
			"notebook_id": "nb-1",
			"version":     "v2",
		},
		Error:     &errMsg,
		IsDeleted: 0,
	}

	return log
}

func assertLLMLogEqual(t *testing.T, want, got schema.LLMLog) {
	t.Helper()

	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.GroupID, got.GroupID)
	assert.Equal(t, want.TraceID, got.TraceID)
	assert.Equal(t, want.UserID, got.UserID)
	assert.Equal(t, want.Scene, got.Scene)
	assert.Equal(t, want.Model, got.Model)
	assert.Equal(t, want.ModelProvider, got.ModelProvider)
	require.NotNil(t, got.ModelParameters)
	assert.Equal(t, *want.ModelParameters, *got.ModelParameters)
	assert.WithinDuration(t, want.CallStartTime, got.CallStartTime, time.Millisecond)
	assert.WithinDuration(t, want.CallFinishTime, got.CallFinishTime, time.Millisecond)
	require.NotNil(t, got.Input)
	assert.Equal(t, *want.Input, *got.Input)
	require.NotNil(t, got.Output)
	assert.Equal(t, *want.Output, *got.Output)
	assert.Equal(t, want.ToolDefinitions, got.ToolDefinitions)
	assert.Equal(t, want.ToolCalls, got.ToolCalls)
	assert.Equal(t, want.UsageDetails, got.UsageDetails)
	require.Len(t, got.CostDetails, len(want.CostDetails))
	for k, wantCost := range want.CostDetails {
		gotCost, ok := got.CostDetails[k]
		require.True(t, ok, "missing cost key %q", k)
		assert.True(t, wantCost.Equal(gotCost), "cost %q: want %s got %s", k, wantCost, gotCost)
	}
	require.NotNil(t, got.TotalCost)
	assert.True(t, want.TotalCost.Equal(*got.TotalCost))
	assert.WithinDuration(t, want.CreateTime, got.CreateTime, time.Millisecond)
	assert.WithinDuration(t, want.UpdateTime, got.UpdateTime, time.Millisecond)
	assert.Equal(t, want.Metadata, got.Metadata)
	require.NotNil(t, got.Error)
	assert.Equal(t, *want.Error, *got.Error)
	assert.Equal(t, want.IsDeleted, got.IsDeleted)
}

func TestLLMLogStoreImpl_Create_NilLog(t *testing.T) {
	s := &LLMLogStoreImpl{}
	require.NoError(t, s.Create(context.Background(), nil))
}

func TestLLMLogStoreImpl_Create_AllFields(t *testing.T) {
	ctx := context.Background()
	store, conn := newStore(t)

	want := fullLLMLog()
	require.NoError(t, store.Create(ctx, want))

	require.NoError(t, conn.Close())

	got, ok := queryLLMLog(t, want.ID)
	require.True(t, ok, "expected persisted row with id=%s", want.ID)
	assertLLMLogEqual(t, *want, got)
}

func TestLLMLogStoreImpl_Create_AssignsDefaultsAndPersists(t *testing.T) {
	ctx := context.Background()
	store, conn := newStore(t)

	log := &schema.LLMLog{
		GroupID: "group-1",
		TraceID: "trace-1",
		UserID:  "user-1",
		Scene:   "chat",
		Model:   "gpt-4",
	}
	require.NoError(t, store.Create(ctx, log))

	assert.NotEmpty(t, log.ID)
	assert.False(t, log.CreateTime.IsZero())
	assert.False(t, log.UpdateTime.IsZero())

	require.NoError(t, conn.Close())

	row, ok := queryLLMLog(t, log.ID)
	require.True(t, ok, "expected persisted row with id=%s", log.ID)
	assert.Equal(t, "group-1", row.GroupID)
	assert.Equal(t, "trace-1", row.TraceID)
	assert.Equal(t, "user-1", row.UserID)
	assert.Equal(t, "chat", row.Scene)
	assert.Equal(t, "gpt-4", row.Model)
}

func TestLLMLogStoreImpl_Create_PreservesExistingFields(t *testing.T) {
	ctx := context.Background()
	store, conn := newStore(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	log := &schema.LLMLog{
		ID:         "fixed-id",
		GroupID:    "group-2",
		TraceID:    "trace-2",
		CreateTime: now,
		UpdateTime: now,
	}
	require.NoError(t, store.Create(ctx, log))

	assert.Equal(t, "fixed-id", log.ID)
	assert.Equal(t, now, log.CreateTime)
	assert.Equal(t, now, log.UpdateTime)

	require.NoError(t, conn.Close())

	row, ok := queryLLMLog(t, "fixed-id")
	require.True(t, ok, "expected persisted row with id=fixed-id")
	assert.Equal(t, "group-2", row.GroupID)
	assert.Equal(t, "trace-2", row.TraceID)
}

func TestLLMLogStoreImpl_Create_ClosedBatcherReturnsErrDatabase(t *testing.T) {
	ctx := context.Background()
	store, conn := newStore(t)

	require.NoError(t, conn.Close())

	err := store.Create(ctx, &schema.LLMLog{GroupID: "group-3"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrDatabase), "expected ErrDatabase, got %v", err)
}
