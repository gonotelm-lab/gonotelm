package postgres

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func newArtifactFixture(notebookId uuid.UUID, status string) *schema.Artifact {
	now := nowMilli()

	return &schema.Artifact{
		Id: uuid.NewV7(), NotebookId: notebookId, UserId: valobj.NewUid(),
		Kind: "mindmap", Status: status, FlowTaskId: uuid.NewV7().String(),
		Payload: []byte(`{}`), CreatedAt: now, UpdatedAt: now,
	}
}

func TestArtifactStoreCRUD(t *testing.T) {
	Convey("ArtifactStore create / upsert / get / list / delete", t, func() {
		store := testArtifactStore
		ctx := t.Context()
		notebookId := uuid.NewV7()

		artifact := newArtifactFixture(notebookId, "pending")
		So(store.Create(ctx, artifact), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", artifact.Id).Delete(&schema.Artifact{}).Error
		})

		Convey("Create never overwrites an existing row", func() {
			So(store.Create(ctx, artifact), ShouldNotBeNil)
		})

		got, err := store.GetById(ctx, artifact.Id)
		So(err, ShouldBeNil)
		So(got.UserId, ShouldEqual, artifact.UserId)
		So(got.Status, ShouldEqual, "pending")

		Convey("UpdateExisting overwrites an existing row", func() {
			artifact.Status = "running"
			artifact.UpdatedAt = nowMilli() + 1000

			affected, err := store.UpdateExisting(ctx, artifact)
			So(err, ShouldBeNil)
			So(affected, ShouldEqual, 1)

			got, err := store.GetById(ctx, artifact.Id)
			So(err, ShouldBeNil)
			So(got.Status, ShouldEqual, "running")
		})

		Convey("UpdateExisting returns 0 and never re-inserts a deleted row", func() {
			So(store.DeleteById(ctx, artifact.Id), ShouldBeNil)

			artifact.Status = "completed"
			affected, err := store.UpdateExisting(ctx, artifact)
			So(err, ShouldBeNil)
			So(affected, ShouldEqual, 0)

			_, err = store.GetById(ctx, artifact.Id)
			So(err, ShouldNotBeNil)
		})

		Convey("ListByNotebookId returns the notebook's artifacts", func() {
			rows, err := store.ListByNotebookId(ctx, notebookId, 10, 0)
			So(err, ShouldBeNil)
			So(rows, ShouldHaveLength, 1)
			So(rows[0].Id, ShouldEqual, artifact.Id)

			empty, err := store.ListByNotebookId(ctx, uuid.NewV7(), 10, 0)
			So(err, ShouldBeNil)
			So(empty, ShouldBeEmpty)
		})

		Convey("ListByStatus returns artifacts with the wanted status", func() {
			rows, err := store.ListByStatus(ctx, []string{"pending"}, 100)
			So(err, ShouldBeNil)
			So(len(rows), ShouldBeGreaterThanOrEqualTo, 1)
		})

		Convey("DeleteById removes only the target row", func() {
			So(store.DeleteById(ctx, artifact.Id), ShouldBeNil)
			_, err := store.GetById(ctx, artifact.Id)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestArtifactStoreDeleteByNotebookId(t *testing.T) {
	Convey("ArtifactStore DeleteByNotebookId", t, func() {
		store := testArtifactStore
		ctx := t.Context()
		targetNotebook, otherNotebook := uuid.NewV7(), uuid.NewV7()

		rows := []*schema.Artifact{
			newArtifactFixture(targetNotebook, "completed"),
			newArtifactFixture(targetNotebook, "running"),
			newArtifactFixture(otherNotebook, "completed"),
		}
		for _, row := range rows {
			So(store.Create(ctx, row), ShouldBeNil)
		}
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).
				Where("notebook_id IN ?", []uuid.UUID{targetNotebook, otherNotebook}).
				Delete(&schema.Artifact{}).Error
		})

		So(store.DeleteByNotebookId(ctx, targetNotebook), ShouldBeNil)

		left, err := store.ListByNotebookId(ctx, targetNotebook, 10, 0)
		So(err, ShouldBeNil)
		So(left, ShouldBeEmpty)

		kept, err := store.ListByNotebookId(ctx, otherNotebook, 10, 0)
		So(err, ShouldBeNil)
		So(kept, ShouldHaveLength, 1)
		So(kept[0].NotebookId, ShouldEqual, otherNotebook)
	})
}
