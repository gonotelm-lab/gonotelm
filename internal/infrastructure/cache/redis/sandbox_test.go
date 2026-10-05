package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSandboxCacheImpl(t *testing.T) {
	Convey("SandboxCache Set/Get/Delete", t, func() {
		ctx := t.Context()
		userId := "test" + uuid.NewV7().String()
		notebookId := "test" + uuid.NewV7().String()
		sandboxId := "sb-" + uuid.NewV7().String()

		err := testSandboxCache.Set(ctx, userId, notebookId, &schema.SandboxDescription{
			Id:      sandboxId,
			Key:     schema.SandboxKey{UserId: userId, NotebookId: notebookId},
			Runtime: "test",
		}, time.Hour)
		So(err, ShouldBeNil)

		got, err := testSandboxCache.Get(ctx, userId, notebookId)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.Id, ShouldEqual, sandboxId)
		So(got.Runtime, ShouldEqual, "test")
		So(got.Key.UserId, ShouldEqual, userId)
		So(got.Key.NotebookId, ShouldEqual, notebookId)

		err = testSandboxCache.Delete(ctx, userId, notebookId)
		So(err, ShouldBeNil)
	})
}

func TestSandboxCacheImplNotFound(t *testing.T) {
	Convey("SandboxCache get missing description", t, func() {
		userId := "test" + uuid.NewV7().String()
		notebookId := "test" + uuid.NewV7().String()

		got, err := testSandboxCache.Get(t.Context(), userId, notebookId)
		So(err, ShouldBeNil)
		So(got, ShouldBeNil)
	})
}
