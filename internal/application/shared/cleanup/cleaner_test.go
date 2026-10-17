package cleanup

import (
	"net/http"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNewCleaner(t *testing.T) {
	Convey("wires the sweeper, scheduler and notebook registrar", t, func() {
		client := &fakeScheduleClient{}
		cleaner := New(Config{
			CallbackBaseURL: "http://notelm:7099",
			Delay:           time.Minute,
		}, Deps{
			ScheduleClient: client,

			SourceRepo:    &fakeSourceRepo{},
			NotebookRepo:  &fakeNotebookRepo{},
			ArtifactRepo:  &fakeArtifactRepo{},
			SourceDocRepo: &fakeSourceDocRepo{},
			ObjectStore:   &fakeObjectStore{},
		})

		So(cleaner.Enabled(), ShouldBeTrue)
		So(cleaner.VerifyCallback(&http.Request{}), ShouldBeNil)
		So(cleaner.RegisterNotebookCleanup(t.Context(), valobj.NewId()), ShouldBeNil)
		So(client.calls, ShouldEqual, 1)
	})
}

func TestNewCleanerDisabledWithoutCallbackURL(t *testing.T) {
	Convey("registers nothing when the callback URL is empty", t, func() {
		client := &fakeScheduleClient{}
		cleaner := New(Config{}, Deps{ScheduleClient: client, SourceRepo: &fakeSourceRepo{}})

		So(cleaner.Enabled(), ShouldBeFalse)
		So(cleaner.ScheduleArtifactCleanup(t.Context(), valobj.NewId(), valobj.NewId(), nil), ShouldBeNil)
		So(client.calls, ShouldEqual, 0)
	})
}
