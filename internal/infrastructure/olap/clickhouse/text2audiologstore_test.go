package clickhouse

import (
	"context"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"
	pkgch "github.com/gonotelm-lab/gonotelm/pkg/clickhouse"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newText2AudioStore(t *testing.T) (*Text2AudioLogStoreImpl, *pkgch.Conn) {
	t.Helper()
	driver, err := openTestConn("default")
	require.NoError(t, err)
	conn := pkgch.NewConn(driver)
	t.Cleanup(func() { _ = conn.Close() })
	store, err := NewText2AudioLogStoreImpl(context.Background(), conn)
	require.NoError(t, err)
	return store, conn
}

func flushText2AudioCreates(t *testing.T, logs ...*schema.Text2AudioLog) {
	t.Helper()
	ctx := context.Background()
	store, conn := newText2AudioStore(t)
	for _, log := range logs {
		require.NoError(t, store.Create(ctx, log))
	}
	require.NoError(t, conn.Close())
}

func TestText2AudioLogStoreImpl_Query_Filters(t *testing.T) {
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Millisecond)
	userID := "aud-query-user"

	both := &schema.Text2AudioLog{
		Id: "aud-both", UserId: userID, GroupId: "g-aud", TraceId: "t-aud",
		Scene: "tts", Model: "tts-1", CallStartTime: base.Add(-time.Minute),
	}
	sameGroup := &schema.Text2AudioLog{
		Id: "aud-same-group", UserId: userID, GroupId: "g-aud", TraceId: "t-other",
		Scene: "tts", Model: "tts-1", CallStartTime: base.Add(-2 * time.Minute),
	}
	sameTrace := &schema.Text2AudioLog{
		Id: "aud-same-trace", UserId: userID, GroupId: "g-other", TraceId: "t-aud",
		Scene: "tts", Model: "tts-1", CallStartTime: base.Add(-3 * time.Minute),
	}
	flushText2AudioCreates(t, both, sameGroup, sameTrace)

	store, _ := newText2AudioStore(t)
	tr := schema.TimeRange{Start: base.Add(-5 * time.Minute), End: base}

	got, err := store.Query(ctx, userID, tr, nil)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "aud-both", got[0].Id)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		GroupId: "g-aud", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		TraceId: "t-aud", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		GroupId: "g-aud", TraceId: "t-aud", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "aud-both", got[0].Id)
}

func TestText2AudioLogStoreImpl_Query_ClosedConnReturnsErrDatabase(t *testing.T) {
	store, conn := newText2AudioStore(t)
	require.NoError(t, conn.Close())
	_, err := store.Query(context.Background(), "u", schema.TimeRange{
		Start: time.Now().Add(-time.Hour), End: time.Now(),
	}, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrDatabase))
}
