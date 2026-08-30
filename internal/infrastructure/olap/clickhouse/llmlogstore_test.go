package clickhouse

import (
	"context"
	"fmt"
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
	driver, err := testDB.OpenConn()
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
		"SELECT * FROM "+schema.LLMLogTableName+" WHERE id = ?", id)
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
		Id:              "full-field-id",
		GroupId:         "group-full",
		TraceId:         "trace-full",
		UserId:          "user-full",
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
		Metadata: map[string]string{
			"notebook_id": "nb-1",
			"version":     "v2",
		},
		Error: &errMsg,
	}

	return log
}

func assertLLMLogEqual(t *testing.T, want, got schema.LLMLog) {
	t.Helper()

	assert.Equal(t, want.Id, got.Id)
	assert.Equal(t, want.GroupId, got.GroupId)
	assert.Equal(t, want.TraceId, got.TraceId)
	assert.Equal(t, want.UserId, got.UserId)
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
	assert.Equal(t, want.Metadata, got.Metadata)
	require.NotNil(t, got.Error)
	assert.Equal(t, *want.Error, *got.Error)
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

	got, ok := queryLLMLog(t, want.Id)
	require.True(t, ok, "expected persisted row with id=%s", want.Id)
	assertLLMLogEqual(t, *want, got)
}

func TestLLMLogStoreImpl_Create_AssignsDefaultsAndPersists(t *testing.T) {
	ctx := context.Background()
	store, conn := newStore(t)

	log := &schema.LLMLog{
		GroupId: "group-1",
		TraceId: "trace-1",
		UserId:  "user-1",
		Scene:   "chat",
		Model:   "gpt-4",
	}
	require.NoError(t, store.Create(ctx, log))

	assert.NotEmpty(t, log.Id)
	assert.False(t, log.CreateTime.IsZero())

	require.NoError(t, conn.Close())

	row, ok := queryLLMLog(t, log.Id)
	require.True(t, ok, "expected persisted row with id=%s", log.Id)
	assert.Equal(t, "group-1", row.GroupId)
	assert.Equal(t, "trace-1", row.TraceId)
	assert.Equal(t, "user-1", row.UserId)
	assert.Equal(t, "chat", row.Scene)
	assert.Equal(t, "gpt-4", row.Model)
}

func TestLLMLogStoreImpl_Create_PreservesExistingFields(t *testing.T) {
	ctx := context.Background()
	store, conn := newStore(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	log := &schema.LLMLog{
		Id:         "fixed-id",
		GroupId:    "group-2",
		TraceId:    "trace-2",
		CreateTime: now,
	}
	require.NoError(t, store.Create(ctx, log))

	assert.Equal(t, "fixed-id", log.Id)
	assert.Equal(t, now, log.CreateTime)

	require.NoError(t, conn.Close())

	row, ok := queryLLMLog(t, "fixed-id")
	require.True(t, ok, "expected persisted row with id=fixed-id")
	assert.Equal(t, "group-2", row.GroupId)
	assert.Equal(t, "trace-2", row.TraceId)
}

func TestLLMLogStoreImpl_Create_ClosedBatcherReturnsErrDatabase(t *testing.T) {
	ctx := context.Background()
	store, conn := newStore(t)

	require.NoError(t, conn.Close())

	err := store.Create(ctx, &schema.LLMLog{GroupId: "group-3"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrDatabase), "expected ErrDatabase, got %v", err)
}

// flushCreates persists logs via Create then closes the write conn so the batcher flushes.
func flushCreates(t *testing.T, logs ...*schema.LLMLog) {
	t.Helper()
	ctx := context.Background()
	store, conn := newStore(t)
	for _, log := range logs {
		require.NoError(t, store.Create(ctx, log))
	}
	require.NoError(t, conn.Close())
}

func TestLLMLogStoreImpl_Query_ByUserAndTimeRange(t *testing.T) {
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Millisecond)
	userID := "query-user-range"

	inRangeOlder := &schema.LLMLog{
		Id:            "q-range-older",
		UserId:        userID,
		GroupId:       "g1",
		TraceId:       "t1",
		Scene:         "chat",
		Model:         "gpt-4",
		CallStartTime: base.Add(-3 * time.Minute),
	}
	inRangeNewer := &schema.LLMLog{
		Id:            "q-range-newer",
		UserId:        userID,
		GroupId:       "g1",
		TraceId:       "t2",
		Scene:         "chat",
		Model:         "gpt-4",
		CallStartTime: base.Add(-1 * time.Minute),
	}
	otherUser := &schema.LLMLog{
		Id:            "q-range-other-user",
		UserId:        "query-user-other",
		GroupId:       "g1",
		TraceId:       "t3",
		Scene:         "chat",
		Model:         "gpt-4",
		CallStartTime: base.Add(-2 * time.Minute),
	}
	outOfRange := &schema.LLMLog{
		Id:            "q-range-out",
		UserId:        userID,
		GroupId:       "g1",
		TraceId:       "t4",
		Scene:         "chat",
		Model:         "gpt-4",
		CallStartTime: base.Add(-10 * time.Minute),
	}
	flushCreates(t, inRangeOlder, inRangeNewer, otherUser, outOfRange)

	store, _ := newStore(t)
	got, err := store.Query(ctx, userID, schema.TimeRange{
		Start: base.Add(-5 * time.Minute),
		End:   base,
	}, nil)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "q-range-newer", got[0].Id)
	assert.Equal(t, "q-range-older", got[1].Id)
}

func TestLLMLogStoreImpl_Query_ExtraGroupAndTrace(t *testing.T) {
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Millisecond)
	userID := "query-user-extra"

	both := &schema.LLMLog{
		Id:            "q-extra-both",
		UserId:        userID,
		GroupId:       "group-extra",
		TraceId:       "trace-extra",
		Scene:         "chat",
		Model:         "gpt-4",
		CallStartTime: base.Add(-time.Minute),
	}
	sameGroupDiffTrace := &schema.LLMLog{
		Id:            "q-extra-same-group",
		UserId:        userID,
		GroupId:       "group-extra",
		TraceId:       "trace-other",
		Scene:         "chat",
		Model:         "gpt-4",
		CallStartTime: base.Add(-2 * time.Minute),
	}
	sameTraceDiffGroup := &schema.LLMLog{
		Id:            "q-extra-same-trace",
		UserId:        userID,
		GroupId:       "group-other",
		TraceId:       "trace-extra",
		Scene:         "chat",
		Model:         "gpt-4",
		CallStartTime: base.Add(-3 * time.Minute),
	}
	flushCreates(t, both, sameGroupDiffTrace, sameTraceDiffGroup)

	store, _ := newStore(t)
	timeRange := schema.TimeRange{
		Start: base.Add(-5 * time.Minute),
		End:   base,
	}

	tests := []struct {
		name    string
		extra   *schema.ExtraQueryConditions
		wantIDs []string
	}{
		{
			name: "group_id only",
			extra: &schema.ExtraQueryConditions{
				GroupId:     "group-extra",
				LimitOffset: schema.LimitOffset{Limit: 10},
			},
			wantIDs: []string{"q-extra-both", "q-extra-same-group"},
		},
		{
			name: "trace_id only",
			extra: &schema.ExtraQueryConditions{
				TraceId:     "trace-extra",
				LimitOffset: schema.LimitOffset{Limit: 10},
			},
			wantIDs: []string{"q-extra-both", "q-extra-same-trace"},
		},
		{
			name: "group_id and trace_id",
			extra: &schema.ExtraQueryConditions{
				GroupId:     "group-extra",
				TraceId:     "trace-extra",
				LimitOffset: schema.LimitOffset{Limit: 10},
			},
			wantIDs: []string{"q-extra-both"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.Query(ctx, userID, timeRange, tt.extra)
			require.NoError(t, err)
			gotIDs := make([]string, len(got))
			for i, log := range got {
				gotIDs[i] = log.Id
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestLLMLogStoreImpl_Query_LimitOffset(t *testing.T) {
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Millisecond)
	userID := "query-user-page"

	logs := make([]*schema.LLMLog, 3)
	for i := 0; i < 3; i++ {
		logs[i] = &schema.LLMLog{
			Id:            fmt.Sprintf("q-page-%d", i),
			UserId:        userID,
			GroupId:       "g-page",
			TraceId:       fmt.Sprintf("t-page-%d", i),
			Scene:         "chat",
			Model:         "gpt-4",
			CallStartTime: base.Add(-time.Duration(3-i) * time.Minute),
		}
	}
	flushCreates(t, logs...)

	store, _ := newStore(t)
	got, err := store.Query(ctx, userID, schema.TimeRange{
		Start: base.Add(-10 * time.Minute),
		End:   base,
	}, &schema.ExtraQueryConditions{
		LimitOffset: schema.LimitOffset{
			Limit:  1,
			Offset: 1,
		},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	// DESC by call_start_time: page-2, page-1, page-0 → offset 1 → page-1
	assert.Equal(t, "q-page-1", got[0].Id)
}

func TestLLMLogStoreImpl_Query_Empty(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)

	got, err := store.Query(ctx, "query-user-none", schema.TimeRange{
		Start: time.Now().UTC().Add(-time.Hour),
		End:   time.Now().UTC(),
	}, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestLLMLogStoreImpl_Query_ClosedConnReturnsErrDatabase(t *testing.T) {
	ctx := context.Background()
	store, conn := newStore(t)
	require.NoError(t, conn.Close())

	_, err := store.Query(ctx, "u", schema.TimeRange{
		Start: time.Now().UTC().Add(-time.Hour),
		End:   time.Now().UTC(),
	}, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrDatabase), "expected ErrDatabase, got %v", err)
}
