package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
	"gorm.io/gorm"
)

func TestSourceStoreCRUD(t *testing.T) {
	Convey("SourceStore CRUD", t, func() {
		db := testDB
		store := testSourceStore
		ctx := t.Context()
		notebookID := createNotebookForSourceTest(t, db)

		content := "source content"
		source := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "new",
			Content:    []byte(content),
			UpdatedAt:  time.Now().UnixMilli(),
		}

		err := store.Create(ctx, source)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = db.WithContext(ctx).Exec(`DELETE FROM sources WHERE id = ?`, source.Id).Error
		})

		got, err := store.GetById(ctx, source.Id)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.Id, ShouldEqual, source.Id)
		So(got.NotebookId, ShouldEqual, source.NotebookId)
		So(strings.TrimSpace(got.Status), ShouldEqual, source.Status)
		So(got.Content, ShouldNotBeNil)
		So(string(got.Content), ShouldEqual, content)

		err = store.UpdateStatus(ctx, source.Id, "done")
		So(err, ShouldBeNil)

		gotAfterUpdate, err := store.GetById(ctx, source.Id)
		So(err, ShouldBeNil)
		So(strings.TrimSpace(gotAfterUpdate.Status), ShouldEqual, "done")

		err = store.DeleteById(ctx, source.Id)
		So(err, ShouldBeNil)

		var count int64
		err = db.WithContext(ctx).Raw(
			`SELECT COUNT(1) FROM sources WHERE id = ?`,
			source.Id,
		).Scan(&count).Error
		So(err, ShouldBeNil)
		So(count, ShouldEqual, int64(0))
	})
}

func TestSourceStoreListAndDeleteByNotebookId(t *testing.T) {
	Convey("SourceStore list and delete by notebook id", t, func() {
		store := testSourceStore
		ctx := t.Context()
		notebookID := createNotebookForSourceTest(t, testDB)

		srcOld := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "new",
			UpdatedAt:  1000,
		}
		srcNew := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "new",
			UpdatedAt:  2000,
		}

		err := store.Create(ctx, srcOld)
		So(err, ShouldBeNil)
		err = store.Create(ctx, srcNew)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM sources WHERE notebook_id = ?`, notebookID).Error
		})

		firstPage, err := store.ListByNotebookId(ctx, notebookID, 1, 0)
		So(err, ShouldBeNil)
		So(len(firstPage), ShouldEqual, 1)
		So(firstPage[0].Id, ShouldEqual, srcNew.Id)

		secondPage, err := store.ListByNotebookId(ctx, notebookID, 1, 1)
		So(err, ShouldBeNil)
		So(len(secondPage), ShouldEqual, 1)
		So(secondPage[0].Id, ShouldEqual, srcOld.Id)

		err = store.DeleteByNotebookId(ctx, notebookID)
		So(err, ShouldBeNil)

		var count int64
		err = testDB.WithContext(ctx).Raw(
			`SELECT COUNT(1) FROM sources WHERE notebook_id = ?`,
			notebookID,
		).Scan(&count).Error
		So(err, ShouldBeNil)
		So(count, ShouldEqual, int64(0))
	})
}

func TestSourceStoreListAndCountIgnoreInited(t *testing.T) {
	Convey("SourceStore list/count should ignore inited status", t, func() {
		store := testSourceStore
		ctx := t.Context()
		notebookID := createNotebookForSourceTest(t, testDB)

		srcPreparing := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "preparing",
			UpdatedAt:  1000,
		}
		srcReady := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "ready",
			UpdatedAt:  2000,
		}
		srcInited := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     schema.SourceStatusInited,
			UpdatedAt:  3000,
		}

		So(store.Create(ctx, srcPreparing), ShouldBeNil)
		So(store.Create(ctx, srcReady), ShouldBeNil)
		So(store.Create(ctx, srcInited), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM sources WHERE notebook_id = ?`, notebookID).Error
		})

		count, err := store.CountByNotebookId(ctx, notebookID)
		So(err, ShouldBeNil)
		So(count, ShouldEqual, int64(2))

		listed, err := store.ListByNotebookId(ctx, notebookID, 10, 0)
		So(err, ShouldBeNil)
		So(len(listed), ShouldEqual, 2)
		So(listed[0].Id, ShouldEqual, srcReady.Id)
		So(listed[1].Id, ShouldEqual, srcPreparing.Id)
	})
}

func TestSourceStoreListByNotebookIdInvalidPagination(t *testing.T) {
	Convey("SourceStore ListByNotebookId invalid pagination", t, func() {
		store := testSourceStore
		ctx := t.Context()
		notebookID := dal.Id(uuid.NewV7())

		_, err := store.ListByNotebookId(ctx, notebookID, 0, 0)
		So(err, ShouldNotBeNil)
		So(strings.Contains(err.Error(), "invalid pagination params"), ShouldBeTrue)

		_, err = store.ListByNotebookId(ctx, notebookID, 1, -1)
		So(err, ShouldNotBeNil)
		So(strings.Contains(err.Error(), "invalid pagination params"), ShouldBeTrue)
	})
}

func TestSourceStoreUpdate(t *testing.T) {
	Convey("SourceStore Update", t, func() {
		store := testSourceStore
		ctx := t.Context()
		notebookID := createNotebookForSourceTest(t, testDB)

		source := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "new",
			Content:    []byte("before update"),
			OwnerId:    "owner_" + uuid.NewV7().String(),
			UpdatedAt:  1000,
		}

		err := store.Create(ctx, source)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM sources WHERE id = ?`, source.Id).Error
		})

		updated := &schema.SourceUpdateParams{
			Id:        source.Id,
			Status:    "ready",
			Content:   []byte("after update"),
			UpdatedAt: 2000,
		}
		err = store.Update(ctx, updated)
		So(err, ShouldBeNil)

		got, err := store.GetById(ctx, source.Id)
		So(err, ShouldBeNil)
		So(strings.TrimSpace(got.Status), ShouldEqual, updated.Status)
		So(string(got.Content), ShouldEqual, string(updated.Content))
		So(got.UpdatedAt, ShouldEqual, updated.UpdatedAt)
	})
}

func TestSourceStoreListByIds(t *testing.T) {
	Convey("SourceStore ListByIds", t, func() {
		store := testSourceStore
		ctx := t.Context()
		notebookID := createNotebookForSourceTest(t, testDB)

		src1 := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "ready",
		}
		src2 := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "ready",
		}

		So(store.Create(ctx, src1), ShouldBeNil)
		So(store.Create(ctx, src2), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM sources WHERE notebook_id = ?`, notebookID).Error
		})

		rows, err := store.ListByIds(ctx, []dal.Id{src1.Id, src2.Id})
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)

		idSet := map[dal.Id]struct{}{}
		for _, row := range rows {
			idSet[row.Id] = struct{}{}
		}
		_, ok1 := idSet[src1.Id]
		_, ok2 := idSet[src2.Id]
		So(ok1, ShouldBeTrue)
		So(ok2, ShouldBeTrue)
	})
}

func TestSourceStoreListByIdsBatches(t *testing.T) {
	Convey("SourceStore ListByIds should work with batches", t, func() {
		store := testSourceStore
		ctx := t.Context()
		notebookID := createNotebookForSourceTest(t, testDB)

		oldBatchSize := sourceIDsQueryBatchSize
		sourceIDsQueryBatchSize = 2
		t.Cleanup(func() {
			sourceIDsQueryBatchSize = oldBatchSize
		})

		src1 := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "ready",
		}
		src2 := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "ready",
		}
		src3 := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "ready",
		}

		So(store.Create(ctx, src1), ShouldBeNil)
		So(store.Create(ctx, src2), ShouldBeNil)
		So(store.Create(ctx, src3), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM sources WHERE notebook_id = ?`, notebookID).Error
		})

		rows, err := store.ListByIds(ctx, []dal.Id{src1.Id, src2.Id, src3.Id})
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 3)

		idSet := map[dal.Id]struct{}{}
		for _, row := range rows {
			idSet[row.Id] = struct{}{}
		}
		_, ok1 := idSet[src1.Id]
		_, ok2 := idSet[src2.Id]
		_, ok3 := idSet[src3.Id]
		So(ok1, ShouldBeTrue)
		So(ok2, ShouldBeTrue)
		So(ok3, ShouldBeTrue)
	})
}

func TestSourceStoreListByIdsEmpty(t *testing.T) {
	Convey("SourceStore ListByIds with empty ids", t, func() {
		store := testSourceStore
		ctx := t.Context()

		rows, err := store.ListByIds(ctx, nil)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 0)
	})
}

func TestSourceStoreListByNotebookIdAndIds(t *testing.T) {
	Convey("SourceStore ListByNotebookIdAndIds", t, func() {
		store := testSourceStore
		ctx := t.Context()

		notebookID := createNotebookForSourceTest(t, testDB)
		otherNotebookID := createNotebookForSourceTest(t, testDB)

		src1 := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "ready",
		}
		src2 := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: notebookID,
			Kind:       "doc",
			Status:     "ready",
		}
		srcOther := &schema.Source{
			Id:         dal.Id(uuid.NewV7()),
			NotebookId: otherNotebookID,
			Kind:       "doc",
			Status:     "ready",
		}

		So(store.Create(ctx, src1), ShouldBeNil)
		So(store.Create(ctx, src2), ShouldBeNil)
		So(store.Create(ctx, srcOther), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM sources WHERE notebook_id = ?`, notebookID).Error
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM sources WHERE notebook_id = ?`, otherNotebookID).Error
		})

		rows, err := store.ListByNotebookIdAndIds(ctx, notebookID, []dal.Id{src1.Id, src2.Id, srcOther.Id})
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)

		idSet := map[dal.Id]struct{}{}
		for _, row := range rows {
			idSet[row.Id] = struct{}{}
			So(row.NotebookId, ShouldEqual, notebookID)
		}
		_, ok1 := idSet[src1.Id]
		_, ok2 := idSet[src2.Id]
		_, okOther := idSet[srcOther.Id]
		So(ok1, ShouldBeTrue)
		So(ok2, ShouldBeTrue)
		So(okOther, ShouldBeFalse)
	})
}

func TestSourceStoreListByNotebookIdAndIdsEmpty(t *testing.T) {
	Convey("SourceStore ListByNotebookIdAndIds with empty ids", t, func() {
		store := testSourceStore
		ctx := t.Context()
		notebookID := dal.Id(uuid.NewV7())

		rows, err := store.ListByNotebookIdAndIds(ctx, notebookID, nil)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 0)
	})
}

func createNotebookForSourceTest(t *testing.T, db *gorm.DB) dal.Id {
	t.Helper()
	ctx := context.Background()
	notebookID := dal.Id(uuid.NewV7())
	tx := db.WithContext(ctx).Exec(
		`INSERT INTO notebooks (id, name, description, owner_id, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		notebookID,
		"nb_for_source_"+uuid.NewV7().String(),
		"for source tests",
		"owner_"+uuid.NewV7().String(),
		time.Now().UnixMilli(),
	)
	if tx.Error != nil {
		t.Fatalf("insert notebook fixture failed: %v", tx.Error)
	}

	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec(`DELETE FROM notebooks WHERE id = ?`, notebookID).Error
	})
	return notebookID
}

func TestSourceGetNotExist(t *testing.T) {
	Convey("SourceGetNotExist", t, func() {
		store := testSourceStore
		ctx := t.Context()
		sourceID := dal.Id(uuid.NewV7())

		_, err := store.GetById(ctx, sourceID)
		t.Log(err)
	})
}
