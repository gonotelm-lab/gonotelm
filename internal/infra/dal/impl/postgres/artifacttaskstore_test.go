package postgres

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
	"gorm.io/gorm"
)

func TestArtifactTaskStoreCreate(t *testing.T) {
	Convey("ArtifactTaskStore Create", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		task := newArtifactTaskFixture(notebookID, "queued", 1000)
		err := store.Create(ctx, task)
		So(err, ShouldBeNil)
		registerArtifactTaskCleanup(t, task.Id)

		var got schema.ArtifactTask
		err = testDB.WithContext(ctx).Where("id = ?", task.Id).Take(&got).Error
		So(err, ShouldBeNil)
		So(normalizeUUID(got.Id), ShouldEqual, normalizeUUID(task.Id))
		So(normalizeUUID(got.NotebookId), ShouldEqual, normalizeUUID(task.NotebookId))
		So(got.Status, ShouldEqual, task.Status)
		So(string(got.Payload), ShouldEqual, string(task.Payload))
	})
}

func TestArtifactTaskStoreGetById(t *testing.T) {
	Convey("ArtifactTaskStore GetById", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		task := newArtifactTaskFixture(notebookID, "queued", 1000)
		mustCreateArtifactTask(t, task)
		registerArtifactTaskCleanup(t, task.Id)

		got, err := store.GetById(ctx, dal.Id(uuid.MustParseString(task.Id)))
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(normalizeUUID(task.Id), ShouldEqual, normalizeUUID(got.Id))
		So(normalizeUUID(task.NotebookId), ShouldEqual, normalizeUUID(got.NotebookId))
		So(got.Kind, ShouldEqual, task.Kind)
		So(got.Status, ShouldEqual, task.Status)
		So(got.UserId, ShouldEqual, task.UserId)
		So(got.CreatedAt, ShouldEqual, task.CreatedAt)
	})
}

func TestArtifactTaskStorePageListByNotebookId(t *testing.T) {
	Convey("ArtifactTaskStore PageListByNotebookId", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()
		otherNotebookID := uuid.NewV7().String()

		task1 := newArtifactTaskFixture(notebookID, "queued", 1000)
		task2 := newArtifactTaskFixture(notebookID, "queued", 2000)
		taskOtherNotebook := newArtifactTaskFixture(otherNotebookID, "queued", 1500)
		mustCreateArtifactTask(t, task1)
		mustCreateArtifactTask(t, task2)
		mustCreateArtifactTask(t, taskOtherNotebook)
		registerArtifactTaskCleanup(t, task1.Id, task2.Id, taskOtherNotebook.Id)

		rows, err := store.PageListByNotebookId(
			ctx,
			dal.Id(uuid.MustParseString(notebookID)),
			dal.Id(uuid.EmptyUUID()),
			10,
		)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)

		expectedIDs := []string{normalizeUUID(task1.Id), normalizeUUID(task2.Id)}
		sort.Strings(expectedIDs)
		So(normalizeUUID(rows[0].Id), ShouldEqual, expectedIDs[0])
		So(normalizeUUID(rows[1].Id), ShouldEqual, expectedIDs[1])

		nextRows, err := store.PageListByNotebookId(
			ctx,
			dal.Id(uuid.MustParseString(notebookID)),
			dal.Id(uuid.MustParseString(expectedIDs[0])),
			10,
		)
		So(err, ShouldBeNil)
		So(len(nextRows), ShouldEqual, 1)
		So(normalizeUUID(nextRows[0].Id), ShouldEqual, expectedIDs[1])
	})
}

func TestArtifactTaskStoreClaimTask(t *testing.T) {
	Convey("ArtifactTaskStore ClaimTask", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		firstPending := newArtifactTaskFixture(notebookID, "queued", 1000)
		secondPending := newArtifactTaskFixture(notebookID, "queued", 2000)
		mustCreateArtifactTask(t, firstPending)
		mustCreateArtifactTask(t, secondPending)
		registerArtifactTaskCleanup(t, firstPending.Id, secondPending.Id)

		claimed, ok, err := store.ClaimTask(
			ctx,
			"queued",
			&schema.ArtifactTaskClaimParams{
				NewStatus: "running",
				UpdatedAt: 3000,
				RunId:     "runner-1",
			},
		)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
		So(claimed, ShouldNotBeNil)

		expectedClaimed := firstPending
		if secondPending.CreatedAt < firstPending.CreatedAt ||
			(secondPending.CreatedAt == firstPending.CreatedAt &&
				normalizeUUID(secondPending.Id) < normalizeUUID(firstPending.Id)) {
			expectedClaimed = secondPending
		}
		So(normalizeUUID(claimed.Id), ShouldEqual, normalizeUUID(expectedClaimed.Id))

		claimedAfterUpdate, err := store.GetById(ctx,
			dal.Id(uuid.MustParseString(expectedClaimed.Id)))
		So(err, ShouldBeNil)
		So(claimedAfterUpdate.Status, ShouldEqual, "running")
		So(claimedAfterUpdate.RunId, ShouldEqual, "runner-1")
		So(claimedAfterUpdate.LockNo, ShouldEqual, expectedClaimed.LockNo+1)
		So(claimedAfterUpdate.UpdatedAt, ShouldEqual, int64(3000))

		noClaimed, noTaskOK, noTaskErr := store.ClaimTask(
			ctx,
			"missing",
			&schema.ArtifactTaskClaimParams{
				NewStatus: "running",
				UpdatedAt: 4000,
				RunId:     "runner-2",
			},
		)
		So(noClaimed, ShouldBeNil)
		So(noTaskOK, ShouldBeFalse)
		So(noTaskErr, ShouldBeNil)
	})
}

func TestArtifactTaskStoreClaimTaskConcurrentContention(t *testing.T) {
	Convey("ArtifactTaskStore ClaimTask concurrent contention", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		pendingTask := newArtifactTaskFixture(notebookID, "queued", 1000)
		mustCreateArtifactTask(t, pendingTask)
		registerArtifactTaskCleanup(t, pendingTask.Id)

		callbackName := "test:artifact_task_claim_block_update_" + uuid.NewV7().String()
		enteredCh := make(chan struct{}, 2)
		releaseCh := make(chan struct{})
		var releaseOnce sync.Once
		releaseUpdate := func() {
			releaseOnce.Do(func() {
				close(releaseCh)
			})
		}
		err := testDB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
			if tx.Statement.Table != (schema.ArtifactTask{}).TableName() {
				return
			}
			enteredCh <- struct{}{}
			<-releaseCh
		})
		So(err, ShouldBeNil)
		t.Cleanup(func() {
			releaseUpdate()
			_ = testDB.Callback().Update().Remove(callbackName)
		})

		type claimResult struct {
			task  *schema.ArtifactTask
			ok    bool
			err   error
			runId string
		}
		resultCh := make(chan claimResult, 2)
		claim := func(runID string) {
			task, ok, err := store.ClaimTask(
				ctx,
				"queued",
				&schema.ArtifactTaskClaimParams{
					NewStatus: "running",
					UpdatedAt: 3000,
					RunId:     runID,
				},
			)
			resultCh <- claimResult{task: task, ok: ok, err: err, runId: runID}
		}

		go claim("runner-a")
		go claim("runner-b")

		waitEntered := func() {
			select {
			case <-enteredCh:
			case <-time.After(3 * time.Second):
				t.Fatalf("wait entered update callback timeout")
			}
		}
		waitResult := func() claimResult {
			select {
			case item := <-resultCh:
				return item
			case <-time.After(3 * time.Second):
				t.Fatalf("wait claim result timeout")
				return claimResult{}
			}
		}

		waitEntered()
		waitEntered()
		releaseUpdate()

		first := waitResult()
		second := waitResult()
		results := []claimResult{first, second}

		successCount := 0
		var winnerRunID string
		for _, item := range results {
			So(item.err, ShouldBeNil)
			if item.ok {
				successCount++
				So(item.task, ShouldNotBeNil)
				winnerRunID = item.runId
				continue
			}
			So(item.task, ShouldBeNil)
		}
		So(successCount, ShouldEqual, 1)

		got, err := store.GetById(ctx, dal.Id(uuid.MustParseString(pendingTask.Id)))
		So(err, ShouldBeNil)
		So(got.Status, ShouldEqual, "running")
		So(got.LockNo, ShouldEqual, int32(1))
		So(got.RunId, ShouldEqual, winnerRunID)
	})
}

func TestArtifactTaskStoreUpdateStatus(t *testing.T) {
	Convey("ArtifactTaskStore UpdateStatus", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		runningTask := newArtifactTaskFixture(notebookID, "running", 1000)
		runningTask.RunId = "runner_" + uuid.NewV7().String()
		mustCreateArtifactTask(t, runningTask)
		registerArtifactTaskCleanup(t, runningTask.Id)

		updated, err := store.UpdateStatus(
			ctx,
			dal.Id(uuid.MustParseString(runningTask.Id)),
			runningTask.RunId,
			"running",
			&schema.ArtifactTaskUpdateStatusParams{
				NewStatus: "completed",
				UpdatedAt: 2000,
			},
		)
		So(err, ShouldBeNil)
		So(updated, ShouldBeTrue)

		got, err := store.GetById(ctx, dal.Id(uuid.MustParseString(runningTask.Id)))
		So(err, ShouldBeNil)
		So(got.Status, ShouldEqual, "completed")
		So(got.RunId, ShouldEqual, runningTask.RunId)
		So(got.UpdatedAt, ShouldEqual, int64(2000))
	})
}

func TestArtifactTaskStoreUpdateStatusNoMatch(t *testing.T) {
	Convey("ArtifactTaskStore UpdateStatus no match", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		task := newArtifactTaskFixture(notebookID, "running", 1000)
		task.RunId = "runner_" + uuid.NewV7().String()
		mustCreateArtifactTask(t, task)
		registerArtifactTaskCleanup(t, task.Id)

		updated, err := store.UpdateStatus(
			ctx,
			dal.Id(uuid.MustParseString(task.Id)),
			"wrong-runner",
			"running",
			&schema.ArtifactTaskUpdateStatusParams{
				NewStatus: "completed",
				UpdatedAt: 2000,
			},
		)
		So(err, ShouldBeNil)
		So(updated, ShouldBeFalse)

		got, err := store.GetById(ctx, dal.Id(uuid.MustParseString(task.Id)))
		So(err, ShouldBeNil)
		So(got.Status, ShouldEqual, "running")
		So(got.RunId, ShouldEqual, task.RunId)
		So(got.UpdatedAt, ShouldEqual, task.UpdatedAt)
	})
}

func TestArtifactTaskStoreUpdateResult(t *testing.T) {
	Convey("ArtifactTaskStore UpdateResult", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		task := newArtifactTaskFixture(notebookID, "running", 1000)
		task.RunId = "runner_" + uuid.NewV7().String()
		mustCreateArtifactTask(t, task)
		registerArtifactTaskCleanup(t, task.Id)

		updated, err := store.UpdateResult(
			ctx,
			dal.Id(uuid.MustParseString(task.Id)),
			task.RunId,
			"running",
			&schema.ArtifactTaskUpdateResultParams{
				NewStatus:  "completed",
				Result:     []byte(`{"mindmap":{"nodes":[]}}`),
				ResultKind: "mindmap_json",
				UpdatedAt:  3000,
			},
		)
		So(err, ShouldBeNil)
		So(updated, ShouldBeTrue)

		got, err := store.GetById(ctx, dal.Id(uuid.MustParseString(task.Id)))
		So(err, ShouldBeNil)
		So(got.Status, ShouldEqual, "completed")
		So(got.ResultKind, ShouldEqual, "mindmap_json")
		So(string(got.Result), ShouldEqual, `{"mindmap":{"nodes":[]}}`)
		So(got.UpdatedAt, ShouldEqual, int64(3000))
	})
}

func TestArtifactTaskStoreUpdateResultNoMatch(t *testing.T) {
	Convey("ArtifactTaskStore UpdateResult no match", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		task := newArtifactTaskFixture(notebookID, "running", 1000)
		task.RunId = "runner_" + uuid.NewV7().String()
		mustCreateArtifactTask(t, task)
		registerArtifactTaskCleanup(t, task.Id)

		updated, err := store.UpdateResult(
			ctx,
			dal.Id(uuid.MustParseString(task.Id)),
			task.RunId,
			"queued",
			&schema.ArtifactTaskUpdateResultParams{
				NewStatus:  "completed",
				Result:     []byte(`{"mindmap":{"nodes":[]}}`),
				ResultKind: "mindmap_json",
				UpdatedAt:  3000,
			},
		)
		So(err, ShouldBeNil)
		So(updated, ShouldBeFalse)

		got, err := store.GetById(ctx, dal.Id(uuid.MustParseString(task.Id)))
		So(err, ShouldBeNil)
		So(got.Status, ShouldEqual, "running")
		So(got.ResultKind, ShouldEqual, "")
		So(got.Result, ShouldBeNil)
		So(got.UpdatedAt, ShouldEqual, task.UpdatedAt)
	})
}

func TestArtifactTaskStoreSetExpiredTasksStatus(t *testing.T) {
	Convey("ArtifactTaskStore SetExpiredTasksStatus", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		targetTask := newArtifactTaskFixture(notebookID, "running", 1000)
		targetTask.ExpiredAt = 2000
		targetTask.UpdatedAt = 1000

		notExpiredTask := newArtifactTaskFixture(notebookID, "running", 1000)
		notExpiredTask.ExpiredAt = 500
		notExpiredTask.UpdatedAt = 1000

		notInIDsTask := newArtifactTaskFixture(notebookID, "running", 1000)
		notInIDsTask.ExpiredAt = 3000
		notInIDsTask.UpdatedAt = 1000

		mustCreateArtifactTask(t, targetTask)
		mustCreateArtifactTask(t, notExpiredTask)
		mustCreateArtifactTask(t, notInIDsTask)
		registerArtifactTaskCleanup(t, targetTask.Id, notExpiredTask.Id, notInIDsTask.Id)

		err := store.SetExpiredTasksStatus(
			ctx,
			[]dal.Id{
				dal.Id(uuid.MustParseString(targetTask.Id)),
				dal.Id(uuid.MustParseString(notExpiredTask.Id)),
			},
			"timeout",
			4000,
			1000,
		)
		So(err, ShouldBeNil)

		gotTarget, err := store.GetById(ctx, dal.Id(uuid.MustParseString(targetTask.Id)))
		So(err, ShouldBeNil)
		So(gotTarget.Status, ShouldEqual, "timeout")
		So(gotTarget.UpdatedAt, ShouldEqual, int64(4000))

		gotNotExpired, err := store.GetById(ctx, dal.Id(uuid.MustParseString(notExpiredTask.Id)))
		So(err, ShouldBeNil)
		So(gotNotExpired.Status, ShouldEqual, "running")
		So(gotNotExpired.UpdatedAt, ShouldEqual, int64(1000))

		gotNotInIDs, err := store.GetById(ctx, dal.Id(uuid.MustParseString(notInIDsTask.Id)))
		So(err, ShouldBeNil)
		So(gotNotInIDs.Status, ShouldEqual, "running")
		So(gotNotInIDs.UpdatedAt, ShouldEqual, int64(1000))
	})
}

func TestArtifactTaskStorePageListExpiredTasks(t *testing.T) {
	Convey("ArtifactTaskStore PageListExpiredTasks", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7().String()

		expiredTask1 := newArtifactTaskFixture(notebookID, "running", 1000)
		expiredTask1.ExpiredAt = 2000
		expiredTask2 := newArtifactTaskFixture(notebookID, "running", 1000)
		expiredTask2.ExpiredAt = 3000
		notExpiredTask := newArtifactTaskFixture(notebookID, "running", 1000)
		notExpiredTask.ExpiredAt = 500

		mustCreateArtifactTask(t, expiredTask1)
		mustCreateArtifactTask(t, expiredTask2)
		mustCreateArtifactTask(t, notExpiredTask)
		registerArtifactTaskCleanup(t, expiredTask1.Id, expiredTask2.Id, notExpiredTask.Id)

		rows, err := store.PageListExpiredTasks(
			ctx,
			dal.Id(uuid.EmptyUUID()),
			10,
			1000,
		)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)

		expectedIDs := []string{normalizeUUID(expiredTask1.Id), normalizeUUID(expiredTask2.Id)}
		sort.Strings(expectedIDs)
		So(normalizeUUID(rows[0].Id), ShouldEqual, expectedIDs[0])
		So(normalizeUUID(rows[1].Id), ShouldEqual, expectedIDs[1])

		nextRows, err := store.PageListExpiredTasks(
			ctx,
			dal.Id(uuid.MustParseString(expectedIDs[0])),
			10,
			1000,
		)
		So(err, ShouldBeNil)
		So(len(nextRows), ShouldEqual, 1)
		So(normalizeUUID(nextRows[0].Id), ShouldEqual, expectedIDs[1])
		So(nextRows[0].ExpiredAt >= 1000, ShouldBeTrue)
	})
}

func newArtifactTaskFixture(notebookID, status string, createdAt int64) *schema.ArtifactTask {
	return &schema.ArtifactTask{
		Id:         uuid.NewV7().String(),
		NotebookId: notebookID,
		Kind:       "mindmap",
		Status:     status,
		ResultKind: "",
		UserId:     "user_" + uuid.NewV7().String(),
		RunId:      "",
		LockNo:     0,
		Payload:    []byte(`{"topic":"rust ownership"}`),
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
		ExpiredAt:  createdAt + 3600000,
	}
}

func mustCreateArtifactTask(t *testing.T, task *schema.ArtifactTask) {
	t.Helper()
	if err := testDB.WithContext(t.Context()).Create(task).Error; err != nil {
		t.Fatalf("insert artifact task fixture failed: %v", err)
	}
}

func normalizeUUID(raw string) string {
	return uuid.MustParseString(raw).String()
}

func registerArtifactTaskCleanup(t *testing.T, ids ...string) {
	t.Helper()
	copiedIDs := append([]string(nil), ids...)
	t.Cleanup(func() {
		if len(copiedIDs) == 0 {
			return
		}
		if err := testDB.WithContext(context.Background()).
			Where("id IN ?", copiedIDs).
			Delete(&schema.ArtifactTask{}).Error; err != nil {
			t.Errorf("cleanup artifact_tasks failed: %v", err)
		}
	})
}
