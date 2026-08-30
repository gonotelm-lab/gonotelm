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

func newEmbeddingStore(t *testing.T) (*EmbeddingLogStoreImpl, *pkgch.Conn) {
	t.Helper()
	driver, err := testDB.OpenConn()
	require.NoError(t, err)
	conn := pkgch.NewConn(driver)
	t.Cleanup(func() { _ = conn.Close() })
	store, err := NewEmbeddingLogStoreImpl(context.Background(), conn)
	require.NoError(t, err)
	return store, conn
}

func flushEmbeddingCreates(t *testing.T, logs ...*schema.EmbeddingLog) {
	t.Helper()
	ctx := context.Background()
	store, conn := newEmbeddingStore(t)
	for _, log := range logs {
		require.NoError(t, store.Create(ctx, log))
	}
	require.NoError(t, conn.Close())
}

func TestEmbeddingLogStoreImpl_Query_Filters(t *testing.T) {
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Millisecond)
	userID := "emb-query-user"

	both := &schema.EmbeddingLog{
		Id: "emb-both", UserId: userID, GroupId: "g-emb", TraceId: "t-emb",
		Scene: "rag", Model: "text-embedding-3", CallStartTime: base.Add(-time.Minute),
	}
	sameGroup := &schema.EmbeddingLog{
		Id: "emb-same-group", UserId: userID, GroupId: "g-emb", TraceId: "t-other",
		Scene: "rag", Model: "text-embedding-3", CallStartTime: base.Add(-2 * time.Minute),
	}
	sameTrace := &schema.EmbeddingLog{
		Id: "emb-same-trace", UserId: userID, GroupId: "g-other", TraceId: "t-emb",
		Scene: "rag", Model: "text-embedding-3", CallStartTime: base.Add(-3 * time.Minute),
	}
	otherUser := &schema.EmbeddingLog{
		Id: "emb-other-user", UserId: "emb-other", GroupId: "g-emb", TraceId: "t-emb",
		Scene: "rag", Model: "text-embedding-3", CallStartTime: base.Add(-time.Minute),
	}
	flushEmbeddingCreates(t, both, sameGroup, sameTrace, otherUser)

	store, _ := newEmbeddingStore(t)
	tr := schema.TimeRange{Start: base.Add(-5 * time.Minute), End: base}

	got, err := store.Query(ctx, userID, tr, nil)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "emb-both", got[0].Id)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		GroupId: "g-emb", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		TraceId: "t-emb", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		GroupId: "g-emb", TraceId: "t-emb", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "emb-both", got[0].Id)
}

func TestEmbeddingLogStoreImpl_Query_ClosedConnReturnsErrDatabase(t *testing.T) {
	store, conn := newEmbeddingStore(t)
	require.NoError(t, conn.Close())
	_, err := store.Query(context.Background(), "u", schema.TimeRange{
		Start: time.Now().Add(-time.Hour), End: time.Now(),
	}, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrDatabase))
}
