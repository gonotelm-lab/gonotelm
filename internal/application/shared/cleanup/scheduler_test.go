package cleanup

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"
	. "github.com/smartystreets/goconvey/convey"
)

type fakeScheduleClient struct {
	calls          int
	fireAt         time.Time
	callbackURL    string
	payload        []byte
	idempotencyKey string
	name           string
	err            error
}

func (f *fakeScheduleClient) RegisterOneShot(
	_ context.Context,
	fireAt time.Time,
	callbackURL string,
	payload []byte,
	idempotencyKey string,
	name string,
) (string, error) {
	f.calls++
	f.fireAt = fireAt
	f.callbackURL = callbackURL
	f.payload = payload
	f.idempotencyKey = idempotencyKey
	f.name = name

	return "schedule-1", f.err
}

func (f *fakeScheduleClient) Verify(*http.Request) error { return nil }

func (f *fakeScheduleClient) Close() error { return nil }

func newTestScheduler(client *fakeScheduleClient, baseURL string) *checkpointScheduler {
	s := newCheckpointScheduler(client, testConfig(baseURL))
	s.now = func() time.Time { return time.Unix(0, 0) }
	s.randFloat = func() float64 { return 0 }

	return s
}

func testConfig(baseURL string) Config {
	return Config{CallbackBaseURL: baseURL, Delay: 30 * time.Minute, Jitter: 10 * time.Minute}
}

func newTestCtx() (context.Context, valobj.Uid, requestid.ID) {
	userId := valobj.NewUid()
	reqId := requestid.Gen()

	return pkgcontext.WithUserId(pkgcontext.WithReqId(context.Background(), reqId), userId), userId, reqId
}

func TestScheduleSourceCleanup(t *testing.T) {
	Convey("source checkpoint URL, key, name and payload", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "http://127.0.0.1:7099/")

		notebookId, sourceId := valobj.NewId(), valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "file/x/y.pdf", false)
		So(err, ShouldBeNil)

		ctx, userId, reqId := newTestCtx()
		So(scheduler.ScheduleSourceCleanup(ctx, notebookId, sourceId, []valobj.StoreKey{key}), ShouldBeNil)

		So(client.calls, ShouldEqual, 1)
		So(client.callbackURL, ShouldEqual, "http://127.0.0.1:7099"+SourceCallbackPath)
		So(client.idempotencyKey, ShouldEqual, KindSource+":"+notebookId.String()+":"+sourceId.String())
		So(client.name, ShouldEqual, userId.String()+"+"+actionDeleteSource+"+"+sourceId.String()+"@"+reqId.String()[:scheduleNameReqIdLen])

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
	Convey("one notebook checkpoint carries sources and artifacts", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "http://notelm:7099")

		notebookId, sourceId, artifactId := valobj.NewId(), valobj.NewId(), valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "artifact/x/y.pptx", false)
		So(err, ShouldBeNil)

		ctx, userId, reqId := newTestCtx()
		scheduler.scheduleNotebookCleanup(ctx, notebookId,
			[]SourceRef{{SourceId: sourceId, Keys: []string{"k"}}},
			[]ArtifactRef{{ArtifactId: artifactId, Keys: []string{key.Encode()}}},
		)

		So(client.calls, ShouldEqual, 1)
		So(client.callbackURL, ShouldEqual, "http://notelm:7099"+NotebookCallbackPath)
		So(client.idempotencyKey, ShouldEqual, KindNotebook+":"+notebookId.String())
		So(client.name, ShouldEqual, userId.String()+"+"+actionDeleteNotebook+"+"+notebookId.String()+"@"+reqId.String()[:scheduleNameReqIdLen])

		payload, err := Unmarshal(client.payload)
		So(err, ShouldBeNil)
		So(payload.Kind, ShouldEqual, KindNotebook)
		So(payload.Sources, ShouldHaveLength, 1)
		So(payload.Sources[0].SourceId, ShouldEqual, sourceId)
		So(payload.Artifacts, ShouldHaveLength, 1)
		So(payload.Artifacts[0].ArtifactId, ShouldEqual, artifactId)
	})
}

func TestScheduleArtifactCleanup(t *testing.T) {
	Convey("single artifact checkpoint URL, key, name and payload", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "http://notelm:7099")

		notebookId, artifactId := valobj.NewId(), valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "artifact/nb/art.pptx", false)
		So(err, ShouldBeNil)

		ctx, userId, reqId := newTestCtx()
		So(scheduler.ScheduleArtifactCleanup(ctx, notebookId, artifactId, []valobj.StoreKey{key}), ShouldBeNil)

		So(client.callbackURL, ShouldEqual, "http://notelm:7099"+ArtifactCallbackPath)
		So(client.idempotencyKey, ShouldEqual, KindArtifact+":"+notebookId.String()+":"+artifactId.String())
		So(client.name, ShouldEqual, userId.String()+"+"+actionDeleteArtifact+"+"+artifactId.String()+"@"+reqId.String()[:scheduleNameReqIdLen])

		payload, err := Unmarshal(client.payload)
		So(err, ShouldBeNil)
		So(payload.Kind, ShouldEqual, KindArtifact)
		So(payload.ArtifactId, ShouldEqual, artifactId)
		So(payload.Keys, ShouldResemble, []string{key.Encode()})
	})
}

func TestScheduleCoverCleanup(t *testing.T) {
	Convey("cover checkpoint URL, key, name, payload and caller-controlled fireAt", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "http://notelm:7099")

		notebookId := valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "notebooks/cover/tok123", true)
		So(err, ShouldBeNil)

		// 调用方传 entity.CoverCleanupDelay
		ctx, userId, reqId := newTestCtx()
		So(scheduler.ScheduleCoverCleanup(ctx, notebookId, []valobj.StoreKey{key}, 24*time.Hour), ShouldBeNil)

		So(client.calls, ShouldEqual, 1)
		So(client.callbackURL, ShouldEqual, "http://notelm:7099"+CoverCallbackPath)
		So(client.name, ShouldEqual, userId.String()+"+"+actionDeleteCover+"+"+notebookId.String()+"@"+reqId.String()[:scheduleNameReqIdLen])

		// 忽略全局 delay/jitter，直接 = now + fireDelay
		So(client.fireAt, ShouldEqual, time.Unix(0, 0).Add(24*time.Hour))

		// idempotencyKey = cover-cleanup:<nb>:<key>:<nonce>，nonce 保证每次注册互相独立
		So(strings.HasPrefix(client.idempotencyKey, KindCover+":"+notebookId.String()+":"+key.Encode()+":"), ShouldBeTrue)

		payload, err := Unmarshal(client.payload)
		So(err, ShouldBeNil)
		So(payload.Kind, ShouldEqual, KindCover)
		So(payload.NotebookId, ShouldEqual, notebookId)
		So(payload.Keys, ShouldResemble, []string{key.Encode()})
	})
}

func TestScheduleCoverCleanupRejectsEmptyKeys(t *testing.T) {
	Convey("a cover checkpoint without valid keys is rejected", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "http://notelm:7099")

		So(scheduler.ScheduleCoverCleanup(t.Context(), valobj.NewId(), nil, 24*time.Hour), ShouldNotBeNil)
		So(client.calls, ShouldEqual, 0)
	})
}

func TestScheduleNameWithoutRequestContext(t *testing.T) {
	Convey("the name omits @reqid when the context has none", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newTestScheduler(client, "http://notelm:7099")

		sourceId := valobj.NewId()
		So(scheduler.ScheduleSourceCleanup(t.Context(), valobj.NewId(), sourceId, nil), ShouldBeNil)
		So(client.name, ShouldEndWith, "+"+actionDeleteSource+"+"+sourceId.String())
		So(client.name, ShouldNotContainSubstring, "@")
	})
}

func TestNextFireAtAppliesJitter(t *testing.T) {
	Convey("jitter stays within [delay, delay+jitter)", t, func() {
		client := &fakeScheduleClient{}
		scheduler := newCheckpointScheduler(client, testConfig("http://notelm:7099"))
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

		err := scheduler.scheduleNotebookCleanup(t.Context(), valobj.NewId(), nil, nil)
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
