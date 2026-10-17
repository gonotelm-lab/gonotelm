package cleanup

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
)

type fakeScheduleClient struct {
	calls          int
	fireAt         time.Time
	callbackURL    string
	payload        []byte
	idempotencyKey string
	err            error
}

func (f *fakeScheduleClient) RegisterOneShot(
	_ context.Context,
	fireAt time.Time,
	callbackURL string,
	payload []byte,
	idempotencyKey string,
) (string, error) {
	f.calls++
	f.fireAt = fireAt
	f.callbackURL = callbackURL
	f.payload = payload
	f.idempotencyKey = idempotencyKey

	return "schedule-1", f.err
}

func (f *fakeScheduleClient) Verify(*http.Request) error { return nil }

func (f *fakeScheduleClient) Close() error { return nil }

func newTestScheduler(client *fakeScheduleClient, baseURL string) *CheckpointScheduler {
	s := NewCheckpointScheduler(client, baseURL, 30*time.Minute, 10*time.Minute)
	s.now = func() time.Time { return time.Unix(0, 0) }
	s.randFloat = func() float64 { return 0 }

	return s
}

func TestSchedulerDisabledWithoutBaseURL(t *testing.T) {
	Convey("does not register without a base URL", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "")

		So(scheduler.Enabled(), ShouldBeFalse)
		So(scheduler.ScheduleSourceCleanup(t.Context(), valobj.NewId(), valobj.NewId(), nil), ShouldBeNil)
		So(client.calls, ShouldEqual, 0)
	})
}

func TestScheduleSourceCleanup(t *testing.T) {
	Convey("source checkpoint URL, idempotency key and payload", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "http://127.0.0.1:7099/")

		notebookId, sourceId := valobj.NewId(), valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "file/x/y.pdf", false)
		So(err, ShouldBeNil)

		So(scheduler.ScheduleSourceCleanup(t.Context(), notebookId, sourceId, []valobj.StoreKey{key}), ShouldBeNil)

		So(client.calls, ShouldEqual, 1)
		So(client.callbackURL, ShouldEqual, "http://127.0.0.1:7099"+SourceCallbackPath)
		So(client.idempotencyKey, ShouldEqual, KindSource+":"+notebookId.String()+":"+sourceId.String())

		// fireAt = now + delay + [0, jitter)
		So(client.fireAt, ShouldEqual, time.Unix(0, 0).Add(30*time.Minute))

		payload, err := Unmarshal(client.payload)
		So(err, ShouldBeNil)
		So(payload.Kind, ShouldEqual, KindSource)
		So(payload.NotebookId, ShouldEqual, notebookId)
		So(payload.SourceId, ShouldEqual, sourceId)
		So(payload.Keys, ShouldResemble, []string{key.Encode()})
	})
}

func TestScheduleNotebookCleanup(t *testing.T) {
	Convey("notebook checkpoint path and payload", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "http://notelm:7099")

		notebookId, sourceId := valobj.NewId(), valobj.NewId()
		refs := []SourceRef{{SourceId: sourceId, Keys: []string{"k"}}}

		So(scheduler.ScheduleNotebookCleanup(t.Context(), notebookId, refs), ShouldBeNil)
		So(client.callbackURL, ShouldEqual, "http://notelm:7099"+NotebookCallbackPath)
		So(client.idempotencyKey, ShouldEqual, KindNotebook+":"+notebookId.String())

		payload, err := Unmarshal(client.payload)
		So(err, ShouldBeNil)
		So(payload.Kind, ShouldEqual, KindNotebook)
		So(payload.Sources, ShouldHaveLength, 1)
		So(payload.Sources[0].SourceId, ShouldEqual, sourceId)
	})
}

func TestNextFireAtAppliesJitter(t *testing.T) {
	Convey("jitter stays within [delay, delay+jitter)", t, func() {
		client := &fakeScheduleClient{}
		scheduler := NewCheckpointScheduler(client, "http://notelm:7099", 30*time.Minute, 10*time.Minute)
		now := time.Unix(0, 0)
		scheduler.now = func() time.Time { return now }

		scheduler.randFloat = func() float64 { return 0 }
		So(scheduler.nextFireAt(), ShouldEqual, now.Add(30*time.Minute))

		scheduler.randFloat = func() float64 { return 1 }
		So(scheduler.nextFireAt(), ShouldEqual, now.Add(40*time.Minute))
	})
}

func TestScheduleRegistrationErrorIsReturned(t *testing.T) {
	Convey("registration failures are returned to the caller", t, func() {
		client := &fakeScheduleClient{err: errors.ErrInner.Msg("flow down")}
		scheduler := newTestScheduler(client, "http://notelm:7099")

		err := scheduler.ScheduleNotebookCleanup(t.Context(), valobj.NewId(), nil)
		So(err, ShouldNotBeNil)
		So(client.calls, ShouldEqual, 1)
	})
}

func TestEncodeKeysSkipsInvalid(t *testing.T) {
	Convey("EncodeKeys skips invalid keys", t, func() {
		key, err := valobj.NewStoreKey(testBucket, "file/x/y.pdf", false)
		So(err, ShouldBeNil)

		encoded := EncodeKeys([]valobj.StoreKey{key, {}})
		So(encoded, ShouldResemble, []string{key.Encode()})
		So(EncodeKeys(nil), ShouldBeNil)
	})
}
