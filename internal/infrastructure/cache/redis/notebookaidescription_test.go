package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNotebookAIDescriptionCacheImpl(t *testing.T) {
	Convey("NotebookAIDescriptionCache Set/Get", t, func() {
		notebookId := uuid.NewV7().String()
		fingerprint := "fp-abc"
		desc := "ai generated summary"

		err := testNotebookAIDescriptionCache.Set(
			t.Context(), notebookId, fingerprint, desc, cache.NotebookAIDescriptionTTL,
		)
		So(err, ShouldBeNil)

		got, err := testNotebookAIDescriptionCache.Get(t.Context(), notebookId, fingerprint)
		So(err, ShouldBeNil)
		So(got, ShouldEqual, desc)

		miss, err := testNotebookAIDescriptionCache.Get(t.Context(), notebookId, "other-fp")
		So(err, ShouldBeNil)
		So(miss, ShouldEqual, "")
	})
}

func TestNotebookAIDescriptionCacheImplMiss(t *testing.T) {
	Convey("NotebookAIDescriptionCache get missing returns empty", t, func() {
		got, err := testNotebookAIDescriptionCache.Get(
			t.Context(), uuid.NewV7().String(), "missing",
		)
		So(err, ShouldBeNil)
		So(got, ShouldEqual, "")
	})
}

func TestNotebookAIDescriptionCacheImplDefaultTTL(t *testing.T) {
	Convey("NotebookAIDescriptionCache uses default TTL when ttl <= 0", t, func() {
		notebookId := uuid.NewV7().String()
		err := testNotebookAIDescriptionCache.Set(
			t.Context(), notebookId, "fp", "desc", time.Duration(0),
		)
		So(err, ShouldBeNil)

		got, err := testNotebookAIDescriptionCache.Get(t.Context(), notebookId, "fp")
		So(err, ShouldBeNil)
		So(got, ShouldEqual, "desc")
	})
}
