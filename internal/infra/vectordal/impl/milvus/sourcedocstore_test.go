package milvus

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
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
