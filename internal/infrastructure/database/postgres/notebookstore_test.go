package postgres

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNotebookStoreCRUD(t *testing.T) {
	Convey("NotebookStore CRUD", t, func() {
		store := testNotebookStore
		ctx := t.Context()

		notebook := &schema.Notebook{
			Id:          database.Id(uuid.NewV7()),
			Name:        "nb_" + uuid.NewV7().String(),
			Description: "initial desc",
			Flags:       1,
			OwnerId:     valobj.NewUid(),
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
		So(gotByID.Flags, ShouldEqual, int32(1))

		notebook.Name = "nb_updated_" + uuid.NewV7().String()
		notebook.Description = "updated desc"
		notebook.Flags = 0
		notebook.UpdatedAt++
		err = store.Update(ctx, notebook)
		So(err, ShouldBeNil)

		gotByName, err := store.GetByNameAndOwnerId(ctx, notebook.Name, notebook.OwnerId)
		So(err, ShouldBeNil)
		So(gotByName, ShouldNotBeNil)
		So(gotByName.Id, ShouldEqual, notebook.Id)
		So(gotByName.Description, ShouldEqual, notebook.Description)
		So(gotByName.Flags, ShouldEqual, int32(0))

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
	Convey("NotebookStore PageListByOwnerId pages and orders owner notebooks", t, func() {
		store := testNotebookStore
		ctx := t.Context()
		ownerID := valobj.NewUid()

		nbOld := &schema.Notebook{
			Id:          uuid.MustParseString("01900000-0000-7000-8000-000000000001"),
			Name:        "nb_old_" + uuid.NewV7().String(),
			Description: "old",
			OwnerId:     ownerID,
			UpdatedAt:   1000,
		}
		nbNew := &schema.Notebook{
			Id:          uuid.MustParseString("01900000-0000-7000-8000-000000000002"),
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

		// updated_at DESC: nbNew 在前，nbOld 在后；同毫秒由 id DESC 兜底。
		rows, err := store.PageListByOwnerId(ctx, ownerID, 10, 0, database.NotebookListOrderLastActive, nil)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].Id, ShouldEqual, nbNew.Id)
		So(rows[1].Id, ShouldEqual, nbOld.Id)

		rows, err = store.PageListByOwnerId(ctx, ownerID, 1, 0, database.NotebookListOrderLastActive, nil)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 1)
		So(rows[0].Id, ShouldEqual, nbNew.Id)

		rows, err = store.PageListByOwnerId(ctx, ownerID, 1, 1, database.NotebookListOrderLastActive, nil)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 1)
		So(rows[0].Id, ShouldEqual, nbOld.Id)

		rows, err = store.PageListByOwnerId(ctx, ownerID, 1, 2, database.NotebookListOrderLastActive, nil)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 0)

		// id ASC 等价于创建时间正序（uuidv7 内嵌时间戳）。
		rows, err = store.PageListByOwnerId(ctx, ownerID, 10, 0, database.NotebookListOrderCreateTime, nil)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].Id, ShouldEqual, nbOld.Id)
		So(rows[1].Id, ShouldEqual, nbNew.Id)

		_, err = store.PageListByOwnerId(ctx, ownerID, 0, 0, database.NotebookListOrderLastActive, nil)
		So(err, ShouldNotBeNil)

		_, err = store.PageListByOwnerId(ctx, ownerID, 1, -1, database.NotebookListOrderLastActive, nil)
		So(err, ShouldNotBeNil)

		_, err = store.PageListByOwnerId(ctx, ownerID, 1, 0, database.NotebookListOrder(99), nil)
		So(err, ShouldNotBeNil)
	})
}

func TestNotebookStoreListByIds(t *testing.T) {
	Convey("NotebookStore ListByIds returns matched rows only", t, func() {
		store := testNotebookStore
		ctx := t.Context()

		nb := &schema.Notebook{
			Id:          database.Id(uuid.NewV7()),
			Name:        "nb_by_ids_" + uuid.NewV7().String(),
			Description: "by ids",
			OwnerId:     valobj.NewUid(),
			UpdatedAt:   nowMilli(),
		}
		So(store.Create(ctx, nb), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", nb.Id).Delete(&schema.Notebook{}).Error
		})

		rows, err := store.ListByIds(ctx, []database.Id{nb.Id, database.Id(uuid.NewV7())})
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 1)
		So(rows[0].Id, ShouldEqual, nb.Id)

		rows, err = store.ListByIds(ctx, nil)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 0)
	})
}

func TestNotebookStoreListByIdsExcludingOwner(t *testing.T) {
	Convey("NotebookStore PageListByIdsExcludingOwner pages, orders and excludes the owner", t, func() {
		store := testNotebookStore
		ctx := t.Context()
		ownerID := valobj.NewUid()
		memberID := valobj.NewUid()

		sharedOlder := &schema.Notebook{
			Id:          uuid.MustParseString("01900000-0000-7000-8000-000000000011"),
			Name:        "nb_shared_older_" + uuid.NewV7().String(),
			Description: "shared older",
			OwnerId:     ownerID,
			UpdatedAt:   1000,
		}
		sharedNewer := &schema.Notebook{
			Id:          uuid.MustParseString("01900000-0000-7000-8000-000000000012"),
			Name:        "nb_shared_newer_" + uuid.NewV7().String(),
			Description: "shared newer",
			OwnerId:     ownerID,
			UpdatedAt:   2000,
		}
		ownNotebook := &schema.Notebook{
			Id:          uuid.MustParseString("01900000-0000-7000-8000-000000000013"),
			Name:        "nb_own_" + uuid.NewV7().String(),
			Description: "owned by member",
			OwnerId:     memberID,
			UpdatedAt:   3000,
		}

		for _, nb := range []*schema.Notebook{sharedOlder, sharedNewer, ownNotebook} {
			So(store.Create(ctx, nb), ShouldBeNil)
		}
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("owner_id IN ?", []valobj.Uid{ownerID, memberID}).
				Delete(&schema.Notebook{}).Error
		})

		ids := []database.Id{sharedOlder.Id, sharedNewer.Id, ownNotebook.Id}

		// 本人拥有的 notebook 即使出现在 ids 里也要排除。
		rows, err := store.PageListByIdsExcludingOwner(ctx, ids, memberID, 10, 0, database.NotebookListOrderLastActive)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].Id, ShouldEqual, sharedNewer.Id)
		So(rows[1].Id, ShouldEqual, sharedOlder.Id)

		rows, err = store.PageListByIdsExcludingOwner(ctx, ids, memberID, 1, 1, database.NotebookListOrderLastActive)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 1)
		So(rows[0].Id, ShouldEqual, sharedOlder.Id)

		rows, err = store.PageListByIdsExcludingOwner(ctx, ids, memberID, 1, 2, database.NotebookListOrderLastActive)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 0)

		rows, err = store.PageListByIdsExcludingOwner(ctx, ids, memberID, 10, 0, database.NotebookListOrderCreateTime)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].Id, ShouldEqual, sharedOlder.Id)
		So(rows[1].Id, ShouldEqual, sharedNewer.Id)

		rows, err = store.PageListByIdsExcludingOwner(ctx, nil, memberID, 10, 0, database.NotebookListOrderLastActive)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 0)

		_, err = store.PageListByIdsExcludingOwner(ctx, ids, memberID, 0, 0, database.NotebookListOrderLastActive)
		So(err, ShouldNotBeNil)

		_, err = store.PageListByIdsExcludingOwner(ctx, ids, memberID, 10, 0, database.NotebookListOrder(99))
		So(err, ShouldNotBeNil)
	})
}

func TestNotebookStoreUpdateName(t *testing.T) {
	Convey("NotebookStore UpdateName", t, func() {
		store := testNotebookStore
		ctx := t.Context()

		notebook := &schema.Notebook{
			Id:          database.Id(uuid.NewV7()),
			Name:        "nb_" + uuid.NewV7().String(),
			Description: "initial desc",
			OwnerId:     valobj.NewUid(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		err := store.Create(ctx, notebook)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", notebook.Id).Delete(&schema.Notebook{}).Error
		})

		newName := "nb_updated_" + uuid.NewV7().String()
		updatedAt := notebook.UpdatedAt + 100
		err = store.UpdateName(ctx, &schema.NotebookUpdateNameParams{
			Id:        notebook.Id,
			Name:      newName,
			UpdatedAt: updatedAt,
		})
		So(err, ShouldBeNil)

		got, err := store.GetById(ctx, notebook.Id)
		So(err, ShouldBeNil)
		So(got.Name, ShouldEqual, newName)
		So(got.Description, ShouldEqual, notebook.Description)
		So(got.UpdatedAt, ShouldEqual, updatedAt)
	})
}

func TestNotebookStoreUpdateDescription(t *testing.T) {
	Convey("NotebookStore UpdateDescription", t, func() {
		store := testNotebookStore
		ctx := t.Context()

		createNotebook := func(desc string, updatedAt int64) *schema.Notebook {
			notebook := &schema.Notebook{
				Id:          database.Id(uuid.NewV7()),
				Name:        "nb_" + uuid.NewV7().String(),
				Description: desc,
				OwnerId:     valobj.NewUid(),
				UpdatedAt:   updatedAt,
			}
			err := store.Create(ctx, notebook)
			So(err, ShouldBeNil)
			t.Cleanup(func() {
				_ = testDB.WithContext(ctx).Where("id = ?", notebook.Id).Delete(&schema.Notebook{}).Error
			})
			return notebook
		}

		Convey("updates description when SkipIfNonEmpty is false", func() {
			notebook := createNotebook("initial desc", 1000)
			err := store.UpdateDescription(ctx, &schema.NotebookUpdateDescriptionParams{
				Id:             notebook.Id,
				Description:    "updated desc",
				SkipIfNonEmpty: false,
				UpdatedAt:      1100,
			})
			So(err, ShouldBeNil)

			got, err := store.GetById(ctx, notebook.Id)
			So(err, ShouldBeNil)
			So(got.Description, ShouldEqual, "updated desc")
			So(got.UpdatedAt, ShouldEqual, int64(1100))
		})

		Convey("updates empty description when SkipIfNonEmpty is true", func() {
			notebook := createNotebook("", 2000)
			err := store.UpdateDescription(ctx, &schema.NotebookUpdateDescriptionParams{
				Id:             notebook.Id,
				Description:    "generated desc",
				SkipIfNonEmpty: true,
				UpdatedAt:      2100,
			})
			So(err, ShouldBeNil)

			got, err := store.GetById(ctx, notebook.Id)
			So(err, ShouldBeNil)
			So(got.Description, ShouldEqual, "generated desc")
			So(got.UpdatedAt, ShouldEqual, int64(2100))
		})

		Convey("does not update non-empty description when SkipIfNonEmpty is true", func() {
			notebook := createNotebook("manual desc", 3000)
			err := store.UpdateDescription(ctx, &schema.NotebookUpdateDescriptionParams{
				Id:             notebook.Id,
				Description:    "generated desc",
				SkipIfNonEmpty: true,
				UpdatedAt:      3100,
			})
			So(err, ShouldBeNil)

			got, err := store.GetById(ctx, notebook.Id)
			So(err, ShouldBeNil)
			So(got.Description, ShouldEqual, "manual desc")
			So(got.UpdatedAt, ShouldEqual, int64(3000))
		})
	})
}

func TestNotebookStoreUpsert(t *testing.T) {
	Convey("NotebookStore Upsert", t, func() {
		store := testNotebookStore
		ctx := t.Context()

		notebookID := database.Id(uuid.NewV7())
		ownerID := valobj.NewUid()

		notebook := &schema.Notebook{
			Id:          notebookID,
			Name:        "nb_upsert_" + uuid.NewV7().String(),
			Description: "initial desc",
			OwnerId:     ownerID,
			UpdatedAt:   1000,
		}

		err := store.Upsert(ctx, notebook)
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", notebookID).Delete(&schema.Notebook{}).Error
		})

		got, err := store.GetById(ctx, notebookID)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.Name, ShouldEqual, notebook.Name)
		So(got.Description, ShouldEqual, notebook.Description)
		So(got.OwnerId, ShouldEqual, ownerID)
		So(got.UpdatedAt, ShouldEqual, int64(1000))

		notebook.Name = "nb_upsert_updated_" + uuid.NewV7().String()
		notebook.Description = "updated desc"
		notebook.OwnerId = valobj.NewUid()
		notebook.UpdatedAt = 2000

		err = store.Upsert(ctx, notebook)
		So(err, ShouldBeNil)

		gotAfterUpsert, err := store.GetById(ctx, notebookID)
		So(err, ShouldBeNil)
		So(gotAfterUpsert, ShouldNotBeNil)
		So(gotAfterUpsert.Name, ShouldEqual, notebook.Name)
		So(gotAfterUpsert.Description, ShouldEqual, "updated desc")
		So(gotAfterUpsert.OwnerId, ShouldEqual, ownerID)
		So(gotAfterUpsert.UpdatedAt, ShouldEqual, int64(2000))
	})
}

func TestNotebookStoreFillNameAndDescriptionIfEmpty(t *testing.T) {
	Convey("NotebookStore FillNameAndDescriptionIfEmpty", t, func() {
		store := testNotebookStore
		ctx := t.Context()

		createNotebook := func(name, desc string, updatedAt int64) *schema.Notebook {
			notebook := &schema.Notebook{
				Id:          database.Id(uuid.NewV7()),
				Name:        name,
				Description: desc,
				OwnerId:     valobj.NewUid(),
				UpdatedAt:   updatedAt,
			}
			err := store.Create(ctx, notebook)
			So(err, ShouldBeNil)
			t.Cleanup(func() {
				_ = testDB.WithContext(ctx).Where("id = ?", notebook.Id).Delete(&schema.Notebook{}).Error
			})
			return notebook
		}

		Convey("updates both fields when both are empty", func() {
			notebook := createNotebook("", "", 1000)
			err := store.FillNameAndDescriptionIfEmpty(ctx, &schema.NotebookFillNameAndDescriptionParams{
				Id:          notebook.Id,
				Name:        "new_name",
				Description: "new_desc",
				UpdatedAt:   1100,
			})
			So(err, ShouldBeNil)

			got, err := store.GetById(ctx, notebook.Id)
			So(err, ShouldBeNil)
			So(got.Name, ShouldEqual, "new_name")
			So(got.Description, ShouldEqual, "new_desc")
			So(got.UpdatedAt, ShouldEqual, int64(1100))
		})

		Convey("keeps non-empty name while updating empty description", func() {
			notebook := createNotebook("manual_name", "", 2000)
			err := store.FillNameAndDescriptionIfEmpty(ctx, &schema.NotebookFillNameAndDescriptionParams{
				Id:          notebook.Id,
				Name:        "generated_name",
				Description: "new_desc",
				UpdatedAt:   2100,
			})
			So(err, ShouldBeNil)

			got, err := store.GetById(ctx, notebook.Id)
			So(err, ShouldBeNil)
			So(got.Name, ShouldEqual, "manual_name")
			So(got.Description, ShouldEqual, "new_desc")
			So(got.UpdatedAt, ShouldEqual, int64(2100))
		})

		Convey("keeps non-empty description while updating empty name", func() {
			notebook := createNotebook("", "manual_desc", 3000)
			err := store.FillNameAndDescriptionIfEmpty(ctx, &schema.NotebookFillNameAndDescriptionParams{
				Id:          notebook.Id,
				Name:        "new_name",
				Description: "generated_desc",
				UpdatedAt:   3100,
			})
			So(err, ShouldBeNil)

			got, err := store.GetById(ctx, notebook.Id)
			So(err, ShouldBeNil)
			So(got.Name, ShouldEqual, "new_name")
			So(got.Description, ShouldEqual, "manual_desc")
			So(got.UpdatedAt, ShouldEqual, int64(3100))
		})

		Convey("keeps both fields and updated_at when both are non-empty", func() {
			notebook := createNotebook("old_name", "old_desc", 1000)
			err := store.FillNameAndDescriptionIfEmpty(ctx, &schema.NotebookFillNameAndDescriptionParams{
				Id:          notebook.Id,
				Name:        "new_name",
				Description: "new_desc",
				UpdatedAt:   1100,
			})
			So(err, ShouldBeNil)

			got, err := store.GetById(ctx, notebook.Id)
			So(err, ShouldBeNil)
			So(got.Name, ShouldEqual, "old_name")
			So(got.Description, ShouldEqual, "old_desc")
			So(got.UpdatedAt, ShouldEqual, int64(1000))
		})
	})
}

func TestNotebookStoreExists(t *testing.T) {
	Convey("NotebookStore Exists", t, func() {
		store := testNotebookStore
		ctx := t.Context()

		exists, err := store.Exists(ctx, database.Id(uuid.NewV7()))
		So(err, ShouldBeNil)
		So(exists, ShouldBeFalse)

		notebook := &schema.Notebook{
			Id:          database.Id(uuid.NewV7()),
			Name:        "nb_" + uuid.NewV7().String(),
			Description: "exists",
			OwnerId:     valobj.NewUid(),
			UpdatedAt:   nowMilli(),
		}
		So(store.Create(ctx, notebook), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("id = ?", notebook.Id).Delete(&schema.Notebook{}).Error
		})

		exists, err = store.Exists(ctx, notebook.Id)
		So(err, ShouldBeNil)
		So(exists, ShouldBeTrue)

		So(store.DeleteById(ctx, notebook.Id), ShouldBeNil)

		exists, err = store.Exists(ctx, notebook.Id)
		So(err, ShouldBeNil)
		So(exists, ShouldBeFalse)
	})
}

func TestNotebookMaintenanceStoreClearNonEmptyDescriptions(t *testing.T) {
	Convey("NotebookMaintenanceStore ClearNonEmptyDescriptions", t, func() {
		store := testNotebookStore
		var maintenance database.NotebookMaintenanceStore = testNotebookStore
		ctx := t.Context()
		ownerID := valobj.NewUid()

		withDesc := &schema.Notebook{
			Id:          database.Id(uuid.NewV7()),
			Name:        "nb_desc_" + uuid.NewV7().String(),
			Description: "legacy",
			OwnerId:     ownerID,
			UpdatedAt:   nowMilli(),
		}
		empty := &schema.Notebook{
			Id:          database.Id(uuid.NewV7()),
			Name:        "nb_empty_" + uuid.NewV7().String(),
			Description: "",
			OwnerId:     ownerID,
			UpdatedAt:   nowMilli(),
		}
		So(store.Create(ctx, withDesc), ShouldBeNil)
		So(store.Create(ctx, empty), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Where("owner_id = ?", ownerID).Delete(&schema.Notebook{}).Error
		})

		count, err := maintenance.CountNonEmptyDescriptions(ctx)
		So(err, ShouldBeNil)
		So(count, ShouldBeGreaterThanOrEqualTo, int64(1))

		err = maintenance.ClearNonEmptyDescriptions(ctx, &schema.NotebookClearDescriptionsParams{
			UpdatedAt: nowMilli() + 1,
		})
		So(err, ShouldBeNil)

		got, err := store.GetById(ctx, withDesc.Id)
		So(err, ShouldBeNil)
		So(got.Description, ShouldEqual, "")

		stillEmpty, err := store.GetById(ctx, empty.Id)
		So(err, ShouldBeNil)
		So(stillEmpty.Description, ShouldEqual, "")
	})
}
