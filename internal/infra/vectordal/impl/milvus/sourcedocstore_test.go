package milvus

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSourceDocStore_GetMetaFromDynamicFields(t *testing.T) {
	Convey("SourceDocStore Get should include dynamic meta fields", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-" + suffix
		sourceID := "test-src-" + suffix
		docID := "test-doc-" + suffix

		cleanupParams := &schema.SourceDocBatchDeleteParams{
			NotebookId: notebookID,
			SourceId:   []string{sourceID},
		}
		_ = store.BatchDelete(ctx, cleanupParams)
		defer func() {
			_ = store.BatchDelete(ctx, cleanupParams)
		}()

		err := store.BatchInsert(ctx, []*schema.SourceDoc{
			{
				Id:         docID,
				NotebookId: notebookID,
				SourceId:   sourceID,
				Content:    "test-content-meta",
				Owner:      "test-owner",
				Embedding:  make([]float32, 1024),
				ChunkPos:   3,
				Meta: map[string]any{
					"test-tag":          "alpha",
					"test-rank":         int64(7),
					schema.FieldContent: "conflict-will-be-ignored",
				},
			},
		})
		So(err, ShouldBeNil)

		var got *schema.SourceDoc
		deadline := time.Now().Add(8 * time.Second)
		for {
			got, err = store.Get(ctx, &schema.SourceDocGetParams{
				NotebookId: notebookID,
				SourceId:   sourceID,
				DocId:      docID,
			})
			if err == nil {
				break
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}

		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.ChunkPos, ShouldEqual, int32(3))
		So(got.Meta, ShouldNotBeNil)
		So(fmt.Sprint(got.Meta["test-tag"]), ShouldEqual, "alpha")
		_, hasRank := got.Meta["test-rank"]
		So(hasRank, ShouldBeTrue)
		_, hasReserved := got.Meta[schema.FieldContent]
		So(hasReserved, ShouldBeFalse)
	})
}

func TestSourceDocStore_GetWithoutMeta(t *testing.T) {
	Convey("SourceDocStore Get should keep Meta nil without dynamic fields", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-" + suffix
		sourceID := "test-src-" + suffix
		docID := "test-doc-" + suffix

		cleanupParams := &schema.SourceDocBatchDeleteParams{
			NotebookId: notebookID,
			SourceId:   []string{sourceID},
		}
		_ = store.BatchDelete(ctx, cleanupParams)
		defer func() {
			_ = store.BatchDelete(ctx, cleanupParams)
		}()

		err := store.BatchInsert(t.Context(), []*schema.SourceDoc{
			{
				Id:         docID,
				NotebookId: notebookID,
				SourceId:   sourceID,
				Content:    "test-content-no-meta",
				Owner:      "test-owner",
				Embedding:  make([]float32, 1024),
				ChunkPos:   2,
			},
		})
		So(err, ShouldBeNil)

		var got *schema.SourceDoc
		deadline := time.Now().Add(8 * time.Second)
		for {
			got, err = store.Get(ctx, &schema.SourceDocGetParams{
				NotebookId: notebookID,
				SourceId:   sourceID,
				DocId:      docID,
			})
			if err == nil {
				break
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}

		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.ChunkPos, ShouldEqual, int32(2))
		So(got.Meta, ShouldBeNil)
	})
}

func TestSourceDocStore_ListWithoutMatches(t *testing.T) {
	Convey("SourceDocStore List should return empty docs without error when no matches", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-not-found-" + suffix
		sourceID := "test-src-not-found-" + suffix

		docs, err := store.List(ctx, &schema.SourceDocListParams{
			NotebookId: notebookID,
			SourceId:   sourceID,
			BatchSize:  16,
		})
		So(err, ShouldBeNil)
		So(docs, ShouldNotBeNil)
		So(len(docs), ShouldEqual, 0)
	})
}

func TestSourceDocStore_ListByChunkPosEmptyChunkPoses(t *testing.T) {
	Convey("SourceDocStore ListByChunkPos should return empty docs when chunk poses are empty", t, func() {
		store := &SourceDocStoreImpl{}
		docs, err := store.ListByChunkPos(t.Context(),
			&schema.SourceDocListByChunkPosParams{
				NotebookId: "test-notebook",
				SourceId:   "test-source",
				ChunkPoses: nil,
				BatchSize:  16,
			})
		So(err, ShouldBeNil)
		So(docs, ShouldNotBeNil)
		So(len(docs), ShouldEqual, 0)
	})
}

func TestSourceDocStore_BatchGetReturnsOrderedDocs(t *testing.T) {
	Convey("SourceDocStore BatchGet should return docs in requested order", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-batch-get-" + suffix
		sourceID := "test-src-batch-get-" + suffix
		docID1 := "test-doc-batch-get-1-" + suffix
		docID2 := "test-doc-batch-get-2-" + suffix

		cleanupParams := &schema.SourceDocBatchDeleteParams{
			NotebookId: notebookID,
			SourceId:   []string{sourceID},
		}
		_ = store.BatchDelete(ctx, cleanupParams)
		defer func() {
			_ = store.BatchDelete(ctx, cleanupParams)
		}()

		err := store.BatchInsert(ctx, []*schema.SourceDoc{
			{
				Id:         docID1,
				NotebookId: notebookID,
				SourceId:   sourceID,
				Content:    "test-content-batch-get-1",
				Owner:      "test-owner",
				Embedding:  make([]float32, 1024),
				ChunkPos:   1,
			},
			{
				Id:         docID2,
				NotebookId: notebookID,
				SourceId:   sourceID,
				Content:    "test-content-batch-get-2",
				Owner:      "test-owner",
				Embedding:  make([]float32, 1024),
				ChunkPos:   2,
			},
		})
		So(err, ShouldBeNil)

		var got []*schema.SourceDoc
		deadline := time.Now().Add(8 * time.Second)
		for {
			got, err = store.BatchGet(ctx, &schema.SourceDocBatchGetParams{
				NotebookId: notebookID,
				SourceId:   sourceID,
				DocIds:     []string{docID2, docID1},
			})
			if err == nil && len(got) == 2 {
				break
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}

		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(len(got), ShouldEqual, 2)
		So(got[0].Id, ShouldEqual, docID2)
		So(got[1].Id, ShouldEqual, docID1)
	})
}

func TestSourceDocStore_BatchGetWithoutMatches(t *testing.T) {
	Convey("SourceDocStore BatchGet should return ErrNoRecord when no matches", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-batch-get-not-found-" + suffix
		sourceID := "test-src-batch-get-not-found-" + suffix
		docID := "test-doc-batch-get-not-found-" + suffix

		docs, err := store.BatchGet(ctx, &schema.SourceDocBatchGetParams{
			NotebookId: notebookID,
			SourceId:   sourceID,
			DocIds:     []string{docID},
		})
		So(docs, ShouldBeNil)
		So(err, ShouldNotBeNil)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestSourceDocStore_GetWithoutMatches(t *testing.T) {
	Convey("SourceDocStore Get should return ErrNoRecord when no matches", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-get-not-found-" + suffix
		sourceID := "test-src-get-not-found-" + suffix
		docID := "test-doc-get-not-found-" + suffix

		doc, err := store.Get(ctx, &schema.SourceDocGetParams{
			NotebookId: notebookID,
			SourceId:   sourceID,
			DocId:      docID,
		})
		So(doc, ShouldBeNil)
		So(err, ShouldNotBeNil)
		So(pkgerrors.Is(err, pkgerrors.ErrNoRecord), ShouldBeTrue)
	})
}

func TestSourceDocStore_QueryWithoutMatches(t *testing.T) {
	Convey("SourceDocStore Query should return empty docs without error when no matches", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-query-no-match-" + suffix
		sourceID := "test-src-query-no-match-" + suffix

		docs, err := store.Query(ctx, &schema.SourceDocQueryParams{
			NotebookId: notebookID,
			SourceIds:  []string{sourceID},
			Target:     "definitely-not-found-target-" + suffix,
			Limit:      8,
		})
		So(err, ShouldBeNil)
		So(docs, ShouldNotBeNil)
		So(len(docs), ShouldEqual, 0)
	})
}

func TestMilvusClient_QueryWithoutMatches(t *testing.T) {
	Convey("Milvus Query should return empty result set without error when no matches", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-query-not-found-" + suffix
		sourceID := "test-src-query-not-found-" + suffix
		filterExpr := fmt.Sprintf(
			`%s == %q && %s == %q`,
			schema.FieldNotebookID, notebookID,
			schema.FieldSourceID, sourceID,
		)

		rs, err := store.cli.Query(ctx, milvusclient.NewQueryOption(collectionName).
			WithPartitions(partitionNameByNotebookID(notebookID)).
			WithFilter(filterExpr).
			WithLimit(16).
			WithOutputFields(schema.OutputFields...))
		So(err, ShouldBeNil)
		So(rs.ResultCount, ShouldEqual, 0)
	})
}

func TestMilvusClient_QueryIteratorWithoutMatches(t *testing.T) {
	Convey("Milvus QueryIterator should return io.EOF when no matches", t, func() {
		store, closeFn := mustNewTestStore(t)
		defer closeFn()

		ctx := t.Context()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		notebookID := "test-nb-query-iter-not-found-" + suffix
		sourceID := "test-src-query-iter-not-found-" + suffix
		filterExpr := fmt.Sprintf(
			`%s == %q && %s == %q`,
			schema.FieldNotebookID, notebookID,
			schema.FieldSourceID, sourceID,
		)

		iter, err := store.cli.QueryIterator(ctx, milvusclient.NewQueryIteratorOption(collectionName).
			WithPartitions(partitionNameByNotebookID(notebookID)).
			WithFilter(filterExpr).
			WithBatchSize(16).
			WithOutputFields(schema.OutputFields...))
		So(err, ShouldBeNil)

		_, err = iter.Next(ctx)
		So(err, ShouldEqual, io.EOF)
	})
}

func mustNewTestStore(t *testing.T) (*SourceDocStoreImpl, func()) {
	t.Helper()

	addr := os.Getenv("ENV_GONOTELM_MILVUS_ADDR")
	if addr == "" {
		t.Skip("skip milvus integration test: ENV_GONOTELM_MILVUS_ADDR is empty")
	}

	username := os.Getenv("ENV_GONOTELM_MILVUS_USERNAME")
	password := os.Getenv("ENV_GONOTELM_MILVUS_PASSWORD")
	dbName := os.Getenv("ENV_GONOTELM_MILVUS_DB_NAME")
	if dbName == "" {
		dbName = "gonotelm"
	}

	cli, err := milvusclient.New(t.Context(), &milvusclient.ClientConfig{
		Address:  addr,
		Username: username,
		Password: password,
		DBName:   dbName,
	})
	So(err, ShouldBeNil)

	store, err := NewSourceDocStoreImpl(cli)
	if err != nil {
		_ = cli.Close(context.Background())
	}
	So(err, ShouldBeNil)

	return store, func() {
		_ = cli.Close(context.Background())
	}
}
