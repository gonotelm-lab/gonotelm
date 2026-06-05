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
		notebookID := uuid.NewV7()

		task := testTask(notebookID, "queued", 1000)
		err := store.Create(ctx, task)
		So(err, ShouldBeNil)
		testCleanupTasks(t, task.Id)

		var got schema.ArtifactTask
		err = testDB.WithContext(ctx).Where("id = ?", task.Id).Take(&got).Error
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, task.Id)
		So(got.NotebookId, ShouldEqual, task.NotebookId)
		So(got.Status, ShouldEqual, task.Status)
		So(string(got.Payload), ShouldEqual, string(task.Payload))
	})
}

func TestArtifactTaskStoreGetById(t *testing.T) {
	Convey("ArtifactTaskStore GetById", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		task := testTask(notebookID, "queued", 1000)
		testCreateTask(t, task)
		testCleanupTasks(t, task.Id)

		got, err := store.GetById(ctx, task.Id)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(task.Id, ShouldEqual, got.Id)
		So(task.NotebookId, ShouldEqual, got.NotebookId)
		So(got.Kind, ShouldEqual, task.Kind)
		So(got.Status, ShouldEqual, task.Status)
		So(got.UserId, ShouldEqual, task.UserId)
		So(got.CreatedAt, ShouldEqual, task.CreatedAt)
	})
}

func TestArtifactTaskStoreGetStatusById(t *testing.T) {
	Convey("ArtifactTaskStore GetStatusById", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		task := testTask(notebookID, "queued", 1000)
		testCreateTask(t, task)
		testCleanupTasks(t, task.Id)

		status, err := store.GetStatusById(ctx, task.Id)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, "queued")
	})
}

func TestArtifactTaskStoreGetStatusByIdNotFound(t *testing.T) {
	Convey("ArtifactTaskStore GetStatusById not found", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()

		_, err := store.GetStatusById(ctx, uuid.NewV7())
		So(err, ShouldNotBeNil)
	})
}

func TestArtifactTaskStorePageListByNotebookId(t *testing.T) {
	Convey("ArtifactTaskStore PageListByNotebookId", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()
		otherNotebookID := uuid.NewV7()

		task1 := testTask(notebookID, "queued", 1000)
		task2 := testTask(notebookID, "queued", 2000)
		taskOtherNotebook := testTask(otherNotebookID, "queued", 1500)
		testCreateTask(t, task1)
		testCreateTask(t, task2)
		testCreateTask(t, taskOtherNotebook)
		testCleanupTasks(t, task1.Id, task2.Id, taskOtherNotebook.Id)

		rows, err := store.PageListByNotebookId(
			ctx,
			dal.Id(notebookID),
			dal.Id(uuid.EmptyUUID()),
			10,
		)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)

		expectedIDs := []uuid.UUID{task1.Id, task2.Id}
		sort.Slice(expectedIDs, func(i, j int) bool {
			return expectedIDs[i].String() < expectedIDs[j].String()
		})
		So(rows[0].Id, ShouldEqual, expectedIDs[0])
		So(rows[1].Id, ShouldEqual, expectedIDs[1])

		nextRows, err := store.PageListByNotebookId(
			ctx,
			dal.Id(notebookID),
			dal.Id(expectedIDs[0]),
			10,
		)
		So(err, ShouldBeNil)
		So(len(nextRows), ShouldEqual, 1)
		So(nextRows[0].Id, ShouldEqual, expectedIDs[1])
	})
}

func TestArtifactTaskStoreListByNotebookId(t *testing.T) {
	Convey("ArtifactTaskStore ListByNotebookId", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()
		otherNotebookID := uuid.NewV7()

		taskOld := testTask(notebookID, "queued", 1000)
		taskNew := testTask(notebookID, "queued", 3000)
		taskMid := testTask(notebookID, "queued", 2000)
		taskOtherNotebook := testTask(otherNotebookID, "queued", 4000)
		testCreateTask(t, taskOld)
		testCreateTask(t, taskNew)
		testCreateTask(t, taskMid)
		testCreateTask(t, taskOtherNotebook)
		testCleanupTasks(t, taskOld.Id, taskNew.Id, taskMid.Id, taskOtherNotebook.Id)

		rows, err := store.ListByNotebookId(
			ctx,
			dal.Id(notebookID),
			2,
			0,
		)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)
		So(rows[0].Id, ShouldEqual, taskNew.Id)
		So(rows[1].Id, ShouldEqual, taskMid.Id)
		So(rows[0].CreatedAt >= rows[1].CreatedAt, ShouldBeTrue)

		nextRows, err := store.ListByNotebookId(
			ctx,
			dal.Id(notebookID),
			2,
			2,
		)
		So(err, ShouldBeNil)
		So(len(nextRows), ShouldEqual, 1)
		So(nextRows[0].Id, ShouldEqual, taskOld.Id)
	})
}

func TestArtifactTaskStoreClaimTask(t *testing.T) {
	Convey("ArtifactTaskStore ClaimTask", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		firstPending := testTask(notebookID, "queued", 1000)
		secondPending := testTask(notebookID, "queued", 2000)
		testCreateTask(t, firstPending)
		testCreateTask(t, secondPending)
		testCleanupTasks(t, firstPending.Id, secondPending.Id)

		claimed, ok, err := store.Claim(
			ctx,
			"queued",
			0,
			&schema.ArtifactTaskClaimParams{
				NewStatus: "running",
				UpdatedAt: 3000,
				RunId:     "runner-1",
				Mode:      0,
			},
		)
		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
		So(claimed, ShouldNotBeNil)

		expectedClaimed := firstPending
		if secondPending.CreatedAt < firstPending.CreatedAt ||
			(secondPending.CreatedAt == firstPending.CreatedAt &&
				secondPending.Id.String() < firstPending.Id.String()) {
			expectedClaimed = secondPending
		}
		So(claimed.Id, ShouldEqual, expectedClaimed.Id)

		claimedAfterUpdate, err := store.GetById(ctx,
			dal.Id(expectedClaimed.Id))
		So(err, ShouldBeNil)
		So(claimedAfterUpdate.Status, ShouldEqual, "running")
		So(claimedAfterUpdate.RunId, ShouldEqual, "runner-1")
		So(claimedAfterUpdate.LockNo, ShouldEqual, expectedClaimed.LockNo+1)
		So(claimedAfterUpdate.UpdatedAt, ShouldEqual, int64(3000))

		noClaimed, noTaskOK, noTaskErr := store.Claim(
			ctx,
			"missing",
			0,
			&schema.ArtifactTaskClaimParams{
				NewStatus: "running",
				UpdatedAt: 4000,
				RunId:     "runner-2",
				Mode:      0,
			},
		)
		So(noClaimed, ShouldBeNil)
		So(noTaskOK, ShouldBeFalse)
		So(noTaskErr, ShouldBeNil)

		// expired_at must be greater than lastExpiredAt.
		expClaim, expOK, expErr := store.Claim(
			ctx,
			"queued",
			secondPending.ExpiredAt,
			&schema.ArtifactTaskClaimParams{
				NewStatus: "running",
				UpdatedAt: 5000,
				RunId:     "runner-3",
				Mode:      0,
			},
		)
		So(expClaim, ShouldBeNil)
		So(expOK, ShouldBeFalse)
		So(expErr, ShouldBeNil)
	})
}

func TestArtifactTaskStoreClaimTaskConcurrentContention(t *testing.T) {
	Convey("ArtifactTaskStore ClaimTask concurrent contention", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		pendingTask := testTask(notebookID, "queued", 1000)
		testCreateTask(t, pendingTask)
		testCleanupTasks(t, pendingTask.Id)

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
			task, ok, err := store.Claim(
				ctx,
				"queued",
				0,
				&schema.ArtifactTaskClaimParams{
					NewStatus: "running",
					UpdatedAt: 3000,
					RunId:     runID,
					Mode:      0,
				},
			)
			resultCh <- claimResult{task: task, ok: ok, err: err, runId: runID}
		}

		go claim("runner-a")
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
		go claim("runner-b")

		// skip-locked mode: when first claimant holds row lock in transaction,
		// second claimant should skip the locked row and return quickly.
		firstResult := waitResult()
		So(firstResult.runId, ShouldEqual, "runner-b")
		So(firstResult.err, ShouldBeNil)
		So(firstResult.ok, ShouldBeFalse)
		So(firstResult.task, ShouldBeNil)

		releaseUpdate()
		secondResult := waitResult()
		So(secondResult.runId, ShouldEqual, "runner-a")
		So(secondResult.err, ShouldBeNil)
		So(secondResult.ok, ShouldBeTrue)
		So(secondResult.task, ShouldNotBeNil)
		So(secondResult.task.Id, ShouldEqual, pendingTask.Id)

		got, err := store.GetById(ctx, dal.Id(pendingTask.Id))
		So(err, ShouldBeNil)
		So(got.Status, ShouldEqual, "running")
		So(got.LockNo, ShouldEqual, int32(1))
		So(got.RunId, ShouldEqual, "runner-a")
	})
}

func TestArtifactTaskStoreClaimTaskConcurrentTwoRowsSkipLock(t *testing.T) {
	Convey("ArtifactTaskStore ClaimTask concurrent claim different rows skip locked", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		firstPending := testTask(notebookID, "queued", 1000)
		secondPending := testTask(notebookID, "queued", 2000)
		testCreateTask(t, firstPending)
		testCreateTask(t, secondPending)
		testCleanupTasks(t, firstPending.Id, secondPending.Id)

		callbackName := "test:artifact_task_claim_block_update_two_rows_skip_locked_" + uuid.NewV7().String()
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
			task, ok, err := store.Claim(
				ctx,
				"queued",
				0,
				&schema.ArtifactTaskClaimParams{
					NewStatus: "running",
					UpdatedAt: 3000,
					RunId:     runID,
					Mode:      0,
				},
			)
			resultCh <- claimResult{task: task, ok: ok, err: err, runId: runID}
		}

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

		go claim("runner-a")
		waitEntered()
		go claim("runner-b")
		waitEntered()
		releaseUpdate()

		firstResult := waitResult()
		secondResult := waitResult()
		So(firstResult.err, ShouldBeNil)
		So(secondResult.err, ShouldBeNil)
		So(firstResult.ok, ShouldBeTrue)
		So(secondResult.ok, ShouldBeTrue)
		So(firstResult.task, ShouldNotBeNil)
		So(secondResult.task, ShouldNotBeNil)
		So(firstResult.task.Id, ShouldNotEqual, secondResult.task.Id)

		claimedIDs := map[uuid.UUID]bool{
			firstResult.task.Id:  true,
			secondResult.task.Id: true,
		}
		So(claimedIDs[firstPending.Id], ShouldBeTrue)
		So(claimedIDs[secondPending.Id], ShouldBeTrue)

		gotFirst, err := store.GetById(ctx, dal.Id(firstPending.Id))
		So(err, ShouldBeNil)
		gotSecond, err := store.GetById(ctx, dal.Id(secondPending.Id))
		So(err, ShouldBeNil)

		So(gotFirst.Status, ShouldEqual, "running")
		So(gotSecond.Status, ShouldEqual, "running")
		So(gotFirst.LockNo, ShouldEqual, int32(1))
		So(gotSecond.LockNo, ShouldEqual, int32(1))

		runIDs := map[string]bool{
			gotFirst.RunId:  true,
			gotSecond.RunId: true,
		}
		So(runIDs["runner-a"], ShouldBeTrue)
		So(runIDs["runner-b"], ShouldBeTrue)
	})
}

func TestArtifactTaskStoreClaimTaskConcurrentVersionLock(t *testing.T) {
	Convey("ArtifactTaskStore ClaimTask concurrent contention version lock mode", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		pendingTask := testTask(notebookID, "queued", 1000)
		testCreateTask(t, pendingTask)
		testCleanupTasks(t, pendingTask.Id)

		callbackName := "test:artifact_task_claim_block_update_version_mode_" + uuid.NewV7().String()
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
			task, ok, err := store.Claim(
				ctx,
				"queued",
				0,
				&schema.ArtifactTaskClaimParams{
					NewStatus: "running",
					UpdatedAt: 3000,
					RunId:     runID,
					Mode:      1,
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

		// version-lock mode: both workers can read the same row then race on update.
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
				So(item.task.Id, ShouldEqual, pendingTask.Id)
				winnerRunID = item.runId
				continue
			}
			So(item.task, ShouldBeNil)
		}
		So(successCount, ShouldEqual, 1)

		got, err := store.GetById(ctx, dal.Id(pendingTask.Id))
		So(err, ShouldBeNil)
		So(got.Status, ShouldEqual, "running")
		So(got.LockNo, ShouldEqual, int32(1))
		So(got.RunId, ShouldEqual, winnerRunID)
	})
}

func TestArtifactTaskStoreSetStatus(t *testing.T) {
	Convey("ArtifactTaskStore SetStatus", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		task := testTask(notebookID, "queued", 1000)
		testCreateTask(t, task)
		testCleanupTasks(t, task.Id)

		err := store.SetStatus(ctx, dal.Id(task.Id), "running", 2000)
		So(err, ShouldBeNil)

		got, err := store.GetById(ctx, dal.Id(task.Id))
		So(err, ShouldBeNil)
		So(got.Status, ShouldEqual, "running")
		So(got.UpdatedAt, ShouldEqual, int64(2000))
		So(got.LockNo, ShouldEqual, int32(0))
		So(got.RunId, ShouldEqual, "")
	})
}

func TestArtifactTaskStoreBatchSetStatus(t *testing.T) {
	Convey("ArtifactTaskStore BatchSetStatus", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		task1 := testTask(notebookID, "queued", 1000)
		task2 := testTask(notebookID, "queued", 2000)
		taskOut := testTask(notebookID, "queued", 3000)
		testCreateTask(t, task1)
		testCreateTask(t, task2)
		testCreateTask(t, taskOut)
		testCleanupTasks(t, task1.Id, task2.Id, taskOut.Id)

		err := store.BatchSetStatus(
			ctx,
			[]dal.Id{dal.Id(task1.Id), dal.Id(task2.Id)},
			"timeout",
			4000,
		)
		So(err, ShouldBeNil)

		got1, err := store.GetById(ctx, dal.Id(task1.Id))
		So(err, ShouldBeNil)
		So(got1.Status, ShouldEqual, "timeout")
		So(got1.UpdatedAt, ShouldEqual, int64(4000))

		got2, err := store.GetById(ctx, dal.Id(task2.Id))
		So(err, ShouldBeNil)
		So(got2.Status, ShouldEqual, "timeout")
		So(got2.UpdatedAt, ShouldEqual, int64(4000))

		gotOut, err := store.GetById(ctx, dal.Id(taskOut.Id))
		So(err, ShouldBeNil)
		So(gotOut.Status, ShouldEqual, "queued")
		So(gotOut.UpdatedAt, ShouldEqual, taskOut.UpdatedAt)
	})
}

func TestArtifactTaskStoreUpdateStatus(t *testing.T) {
	Convey("ArtifactTaskStore UpdateStatus", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		runningTask := testTask(notebookID, "running", 1000)
		runningTask.RunId = "runner_" + uuid.NewV7().String()[:16]
		testCreateTask(t, runningTask)
		testCleanupTasks(t, runningTask.Id)

		updated, err := store.UpdateStatus(
			ctx,
			dal.Id(runningTask.Id),
			runningTask.RunId,
			"running",
			&schema.ArtifactTaskUpdateStatusParams{
				NewStatus: "completed",
				UpdatedAt: 2000,
			},
		)
		So(err, ShouldBeNil)
		So(updated, ShouldBeTrue)

		got, err := store.GetById(ctx, dal.Id(runningTask.Id))
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
		notebookID := uuid.NewV7()

		task := testTask(notebookID, "running", 1000)
		task.RunId = "runner_" + uuid.NewV7().String()[:16]
		testCreateTask(t, task)
		testCleanupTasks(t, task.Id)

		updated, err := store.UpdateStatus(
			ctx,
			dal.Id(task.Id),
			"wrong-runner",
			"running",
			&schema.ArtifactTaskUpdateStatusParams{
				NewStatus: "completed",
				UpdatedAt: 2000,
			},
		)
		So(err, ShouldBeNil)
		So(updated, ShouldBeFalse)

		got, err := store.GetById(ctx, dal.Id(task.Id))
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
		notebookID := uuid.NewV7()

		task := testTask(notebookID, "running", 1000)
		task.RunId = "runner_" + uuid.NewV7().String()[:16]
		testCreateTask(t, task)
		testCleanupTasks(t, task.Id)

		updated, err := store.UpdateResult(
			ctx,
			dal.Id(task.Id),
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

		got, err := store.GetById(ctx, dal.Id(task.Id))
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
		notebookID := uuid.NewV7()

		task := testTask(notebookID, "running", 1000)
		task.RunId = "runner_" + uuid.NewV7().String()[:16]
		testCreateTask(t, task)
		testCleanupTasks(t, task.Id)

		updated, err := store.UpdateResult(
			ctx,
			dal.Id(task.Id),
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

		got, err := store.GetById(ctx, dal.Id(task.Id))
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
		notebookID := uuid.NewV7()

		expInIDs := testTask(notebookID, "running", 1000)
		expInIDs.ExpiredAt = 500
		expInIDs.UpdatedAt = 1000

		aliveTask := testTask(notebookID, "running", 1000)
		aliveTask.ExpiredAt = 2000
		aliveTask.UpdatedAt = 1000

		expOutIDs := testTask(notebookID, "running", 1000)
		expOutIDs.ExpiredAt = 300
		expOutIDs.UpdatedAt = 1000

		testCreateTask(t, expInIDs)
		testCreateTask(t, aliveTask)
		testCreateTask(t, expOutIDs)
		testCleanupTasks(t, expInIDs.Id, aliveTask.Id, expOutIDs.Id)

		err := store.SetExpiredTasksStatus(
			ctx,
			[]dal.Id{
				dal.Id(expInIDs.Id),
				dal.Id(aliveTask.Id),
			},
			"timeout",
			4000,
			1000,
		)
		So(err, ShouldBeNil)

		gotExpIn, err := store.GetById(ctx, dal.Id(expInIDs.Id))
		So(err, ShouldBeNil)
		So(gotExpIn.Status, ShouldEqual, "timeout")
		So(gotExpIn.UpdatedAt, ShouldEqual, int64(4000))

		gotAlive, err := store.GetById(ctx, dal.Id(aliveTask.Id))
		So(err, ShouldBeNil)
		So(gotAlive.Status, ShouldEqual, "running")
		So(gotAlive.UpdatedAt, ShouldEqual, int64(1000))

		gotExpOut, err := store.GetById(ctx, dal.Id(expOutIDs.Id))
		So(err, ShouldBeNil)
		So(gotExpOut.Status, ShouldEqual, "running")
		So(gotExpOut.UpdatedAt, ShouldEqual, int64(1000))
	})
}

func TestArtifactTaskStorePageListExpiredTasks(t *testing.T) {
	Convey("ArtifactTaskStore PageListExpiredTasks", t, func() {
		store := testArtifactTaskStore
		ctx := t.Context()
		notebookID := uuid.NewV7()

		expiredTask1 := testTask(notebookID, "running", 1000)
		expiredTask1.ExpiredAt = 500
		expiredTask2 := testTask(notebookID, "running", 1000)
		expiredTask2.ExpiredAt = 1000
		notExpiredTask := testTask(notebookID, "running", 1000)
		notExpiredTask.ExpiredAt = 3000

		testCreateTask(t, expiredTask1)
		testCreateTask(t, expiredTask2)
		testCreateTask(t, notExpiredTask)
		testCleanupTasks(t, expiredTask1.Id, expiredTask2.Id, notExpiredTask.Id)

		rows, err := store.PageListExpiredTasks(
			ctx,
			dal.Id(uuid.EmptyUUID()),
			10,
			1000,
		)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)

		expectedIDs := []uuid.UUID{expiredTask1.Id, expiredTask2.Id}
		sort.Slice(expectedIDs, func(i, j int) bool {
			return expectedIDs[i].String() < expectedIDs[j].String()
		})
		So(rows[0].Id, ShouldEqual, expectedIDs[0])
		So(rows[1].Id, ShouldEqual, expectedIDs[1])

		nextRows, err := store.PageListExpiredTasks(
			ctx,
			dal.Id(expectedIDs[0]),
			10,
			1000,
		)
		So(err, ShouldBeNil)
		So(len(nextRows), ShouldEqual, 1)
		So(nextRows[0].Id, ShouldEqual, expectedIDs[1])
		So(nextRows[0].ExpiredAt <= 1000, ShouldBeTrue)
	})
}

func testTask(notebookID uuid.UUID, status string, createdAt int64) *schema.ArtifactTask {
	return &schema.ArtifactTask{
		Id:         uuid.NewV7(),
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

func testCreateTask(t *testing.T, task *schema.ArtifactTask) {
	t.Helper()
	if err := testDB.WithContext(t.Context()).Create(task).Error; err != nil {
		t.Fatalf("insert artifact task fixture failed: %v", err)
	}
}

func testCleanupTasks(t *testing.T, ids ...uuid.UUID) {
	t.Helper()
	copiedIDs := append([]uuid.UUID(nil), ids...)
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
