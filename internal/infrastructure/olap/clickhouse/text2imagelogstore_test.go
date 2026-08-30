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

func newText2ImageStore(t *testing.T) (*Text2ImageLogStoreImpl, *pkgch.Conn) {
	t.Helper()
	driver, err := openTestConn("default")
	require.NoError(t, err)
	conn := pkgch.NewConn(driver)
	t.Cleanup(func() { _ = conn.Close() })
	store, err := NewText2ImageLogStoreImpl(context.Background(), conn)
	require.NoError(t, err)
	return store, conn
}

func flushText2ImageCreates(t *testing.T, logs ...*schema.Text2ImageLog) {
	t.Helper()
	ctx := context.Background()
	store, conn := newText2ImageStore(t)
	for _, log := range logs {
		require.NoError(t, store.Create(ctx, log))
	}
	require.NoError(t, conn.Close())
}

func TestText2ImageLogStoreImpl_Query_Filters(t *testing.T) {
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Millisecond)
	userID := "img-query-user"

	both := &schema.Text2ImageLog{
		Id: "img-both", UserId: userID, GroupId: "g-img", TraceId: "t-img",
		Scene: "gen", Model: "dall-e-3", CallStartTime: base.Add(-time.Minute),
	}
	sameGroup := &schema.Text2ImageLog{
		Id: "img-same-group", UserId: userID, GroupId: "g-img", TraceId: "t-other",
		Scene: "gen", Model: "dall-e-3", CallStartTime: base.Add(-2 * time.Minute),
	}
	sameTrace := &schema.Text2ImageLog{
		Id: "img-same-trace", UserId: userID, GroupId: "g-other", TraceId: "t-img",
		Scene: "gen", Model: "dall-e-3", CallStartTime: base.Add(-3 * time.Minute),
	}
	flushText2ImageCreates(t, both, sameGroup, sameTrace)

	store, _ := newText2ImageStore(t)
	tr := schema.TimeRange{Start: base.Add(-5 * time.Minute), End: base}

	got, err := store.Query(ctx, userID, tr, nil)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "img-both", got[0].Id)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		GroupId: "g-img", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		TraceId: "t-img", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)

	got, err = store.Query(ctx, userID, tr, &schema.ExtraQueryConditions{
		GroupId: "g-img", TraceId: "t-img", LimitOffset: schema.LimitOffset{Limit: 10},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "img-both", got[0].Id)
}

func TestText2ImageLogStoreImpl_Query_ClosedConnReturnsErrDatabase(t *testing.T) {
	store, conn := newText2ImageStore(t)
	require.NoError(t, conn.Close())
	_, err := store.Query(context.Background(), "u", schema.TimeRange{
		Start: time.Now().Add(-time.Hour), End: time.Now(),
	}, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrDatabase))
}
