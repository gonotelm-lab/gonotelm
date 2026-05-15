package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNotebookStoreCRUD(t *testing.T) {
	Convey("NotebookStore CRUD", t, func() {
		store := testNotebookStore
		ctx := t.Context()

		notebook := &schema.Notebook{
			Id:          dal.Id(uuid.NewV7()),
			Name:        "nb_" + uuid.NewV7().String(),
			Description: "initial desc",
			OwnerId:     "owner_" + uuid.NewV7().String(),
			UpdatedAt:   time.Now().UnixMilli(),
		}

		err := store.Create(ctx, notebook)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", notebook.Id).Delete(&schema.Notebook{}).Error
		})

		gotByID, err := store.GetById(ctx, notebook.Id)
		So(err, ShouldBeNil)
		So(gotByID, ShouldNotBeNil)
		So(gotByID.Id, ShouldEqual, notebook.Id)
		So(gotByID.Name, ShouldEqual, notebook.Name)
		So(gotByID.OwnerId, ShouldEqual, notebook.OwnerId)
		So(gotByID.Description, ShouldEqual, notebook.Description)

		notebook.Name = "nb_updated_" + uuid.NewV7().String()
		notebook.Description = "updated desc"
		notebook.UpdatedAt++
		err = store.Update(ctx, notebook)
		So(err, ShouldBeNil)

		gotByName, err := store.GetByNameAndOwnerId(ctx, notebook.Name, notebook.OwnerId)
		So(err, ShouldBeNil)
		So(gotByName, ShouldNotBeNil)
		So(gotByName.Id, ShouldEqual, notebook.Id)
		So(gotByName.Description, ShouldEqual, notebook.Description)

		err = store.DeleteById(ctx, notebook.Id)
		So(err, ShouldBeNil)

		var count int64
		err = testDB.WithContext(ctx).
			Model(&schema.Notebook{}).
			Where("id = ?", notebook.Id).
			Count(&count).Error
		So(err, ShouldBeNil)
		So(count, ShouldEqual, int64(0))
	})
}

func TestNotebookStoreListByOwnerId(t *testing.T) {
	Convey("NotebookStore ListByOwnerId", t, func() {
		store := testNotebookStore
		ctx := t.Context()
		ownerID := "owner_" + uuid.NewV7().String()

		nbOld := &schema.Notebook{
			Id:          dal.Id(uuid.NewV7()),
			Name:        "nb_old_" + uuid.NewV7().String(),
			Description: "old",
			OwnerId:     ownerID,
			UpdatedAt:   1000,
		}
		nbNew := &schema.Notebook{
			Id:          dal.Id(uuid.NewV7()),
			Name:        "nb_new_" + uuid.NewV7().String(),
			Description: "new",
			OwnerId:     ownerID,
			UpdatedAt:   2000,
		}

		err := store.Create(ctx, nbOld)
		So(err, ShouldBeNil)
		err = store.Create(ctx, nbNew)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("owner_id = ?", ownerID).Delete(&schema.Notebook{}).Error
		})

		firstPage, err := store.ListByOwnerId(ctx, ownerID, 1, 0, 1)
		So(err, ShouldBeNil)
		So(len(firstPage), ShouldEqual, 1)
		So(firstPage[0].Id, ShouldEqual, nbNew.Id)

		secondPage, err := store.ListByOwnerId(ctx, ownerID, 1, 1, 1)
		So(err, ShouldBeNil)
		So(len(secondPage), ShouldEqual, 1)
		So(secondPage[0].Id, ShouldEqual, nbOld.Id)
	})
}

func TestNotebookStoreListByOwnerIdInvalidPagination(t *testing.T) {
	Convey("NotebookStore ListByOwnerId invalid pagination", t, func() {
		store := testNotebookStore
		ctx := t.Context()
		ownerID := "owner_" + uuid.NewV7().String()

		_, err := store.ListByOwnerId(ctx, ownerID, 0, 0, 1)
		So(err, ShouldNotBeNil)
		So(strings.Contains(err.Error(), "invalid pagination params"), ShouldBeTrue)

		_, err = store.ListByOwnerId(ctx, ownerID, 1, -1, 1)
		So(err, ShouldNotBeNil)
		So(strings.Contains(err.Error(), "invalid pagination params"), ShouldBeTrue)
	})
}
