package redis

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	cacheerrors "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/errors"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"

	goredis "github.com/redis/go-redis/v9"
)

// 复用实现里的 key 生成逻辑，避免测试里硬编码 key 格式。
func testChatMessageStreamCacheImpl() *ChatMessageStreamCacheImpl {
	impl, ok := testChatMessageStreamCache.(*ChatMessageStreamCacheImpl)
	if !ok {
		panic("unexpected chat message stream cache impl")
	}
	return impl
}

func newTestTask(userId, chatId string) *schema.ChatMessageTask {
	now := time.Now().Unix()
	return &schema.ChatMessageTask{
		Id:             uuid.NewV4().String(),
		Status:         "running",
		CreatedAt:      now,
		UpdatedAt:      now,
		ChatId:         chatId,
		SourceIds:      []string{"src-1", "src-2"},
		UserId:         userId,
		ExpireDuration: time.Minute,
	}
}

func TestChatMessageStreamCache_SetGetDeleteTask(t *testing.T) {
	Convey("ChatMessageStreamCache Set/Get/Delete task", t, func() {
		ctx := t.Context()
		task := newTestTask("user-"+uuid.NewV4().String(), "chat-"+uuid.NewV4().String())

		taskId, err := testChatMessageStreamCache.SetTask(ctx, task)
		So(err, ShouldBeNil)
		So(taskId, ShouldEqual, task.Id)
		defer testChatMessageStreamCache.DeleteTask(ctx, taskId)

		got, err := testChatMessageStreamCache.GetTask(ctx, taskId)
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, task.Id)
		So(got.UserId, ShouldEqual, task.UserId)
		So(got.ChatId, ShouldEqual, task.ChatId)
		So(got.Status, ShouldEqual, task.Status)
		So(got.SourceIds, ShouldHaveLength, 2)
		So(got.SourceIds[0], ShouldEqual, "src-1")

		byUserChat, err := testChatMessageStreamCache.GetTaskByUserAndChatId(ctx, task.UserId, task.ChatId)
		So(err, ShouldBeNil)
		So(byUserChat.Id, ShouldEqual, task.Id)

		err = testChatMessageStreamCache.DeleteTask(ctx, taskId)
		So(err, ShouldBeNil)

		_, err = testChatMessageStreamCache.GetTask(ctx, taskId)
		So(errors.Is(err, cacheerrors.ErrTaskNotFound), ShouldBeTrue)

		_, err = testChatMessageStreamCache.GetTaskByUserAndChatId(ctx, task.UserId, task.ChatId)
		So(errors.Is(err, cacheerrors.ErrTaskNotFound), ShouldBeTrue)
	})
}

func TestChatMessageStreamCache_SetTask_AutoGenerateId(t *testing.T) {
	Convey("ChatMessageStreamCache SetTask auto-generates id", t, func() {
		ctx := t.Context()
		task := newTestTask("user-"+uuid.NewV4().String(), "chat-"+uuid.NewV4().String())
		task.Id = ""

		taskId, err := testChatMessageStreamCache.SetTask(ctx, task)
		So(err, ShouldBeNil)
		defer testChatMessageStreamCache.DeleteTask(ctx, taskId)

		So(taskId, ShouldNotBeEmpty)
		So(task.Id, ShouldEqual, taskId)

		got, err := testChatMessageStreamCache.GetTask(ctx, taskId)
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, taskId)
	})
}

func TestChatMessageStreamCache_GetTask_NotFound(t *testing.T) {
	Convey("ChatMessageStreamCache GetTask not found", t, func() {
		_, err := testChatMessageStreamCache.GetTask(t.Context(), "missing-task-"+uuid.NewV4().String())
		So(errors.Is(err, cacheerrors.ErrTaskNotFound), ShouldBeTrue)
	})
}

func TestChatMessageStreamCache_GetTaskByUserAndChatId_NotFound(t *testing.T) {
	Convey("ChatMessageStreamCache GetTaskByUserAndChatId not found", t, func() {
		_, err := testChatMessageStreamCache.GetTaskByUserAndChatId(
			t.Context(),
			"missing-user-"+uuid.NewV4().String(),
			"missing-chat-"+uuid.NewV4().String(),
		)
		So(errors.Is(err, cacheerrors.ErrTaskNotFound), ShouldBeTrue)
	})
}

func TestChatMessageStreamCache_DeleteTask_NotFound(t *testing.T) {
	Convey("ChatMessageStreamCache DeleteTask not found", t, func() {
		err := testChatMessageStreamCache.DeleteTask(t.Context(), "missing-task-"+uuid.NewV4().String())
		So(errors.Is(err, cacheerrors.ErrTaskNotFound), ShouldBeTrue)
	})
}

func TestChatMessageStreamCache_SetTask_OverwriteUserChatMapping(t *testing.T) {
	Convey("ChatMessageStreamCache overwrites user/chat mapping", t, func() {
		ctx := t.Context()
		userId := "user-" + uuid.NewV4().String()
		chatId := "chat-" + uuid.NewV4().String()

		first := newTestTask(userId, chatId)
		firstId, err := testChatMessageStreamCache.SetTask(ctx, first)
		So(err, ShouldBeNil)
		defer testChatMessageStreamCache.DeleteTask(ctx, firstId)

		second := newTestTask(userId, chatId)
		secondId, err := testChatMessageStreamCache.SetTask(ctx, second)
		So(err, ShouldBeNil)
		defer testChatMessageStreamCache.DeleteTask(ctx, secondId)

		got, err := testChatMessageStreamCache.GetTaskByUserAndChatId(ctx, userId, chatId)
		So(err, ShouldBeNil)
		So(got.Id, ShouldEqual, secondId)
	})
}

func TestChatMessageStreamCache_PullEventStream(t *testing.T) {
	Convey("ChatMessageStreamCache PullEventStream", t, func() {
		ctx := t.Context()
		testKey := "test-stream-" + uuid.NewV4().String()
		defer testChatMessageStreamCache.DeleteEventStream(ctx, testKey)

		count := 5
		for idx := range count {
			_, err := testChatMessageStreamCache.AppendEventStream(
				ctx,
				testKey,
				&schema.ChatMessageStreamEvent{
					Data: []byte("test-data-" + strconv.Itoa(idx)),
				})
			So(err, ShouldBeNil)
		}

		events, err := testChatMessageStreamCache.PullEventStream(
			ctx,
			testKey,
			schema.PullEventStreamArgs{},
		)
		So(err, ShouldBeNil)
		So(events, ShouldHaveLength, count)

		for idx, event := range events {
			So(string(event.Data), ShouldEqual, "test-data-"+strconv.Itoa(idx))
		}
	})
}

func TestChatMessageStreamCache_PullEventStream_WithLastId(t *testing.T) {
	Convey("ChatMessageStreamCache PullEventStream with LastId", t, func() {
		ctx := t.Context()
		testKey := "test-stream-" + uuid.NewV4().String()
		defer testChatMessageStreamCache.DeleteEventStream(ctx, testKey)

		count := 5
		lastId := ""
		ids := make([]string, 0, count)
		for idx := range count {
			id, err := testChatMessageStreamCache.AppendEventStream(
				ctx,
				testKey,
				&schema.ChatMessageStreamEvent{
					Data: []byte("test-data-" + strconv.Itoa(idx)),
				})
			So(err, ShouldBeNil)
			if idx == 2 {
				lastId = id
			}
			ids = append(ids, id)
		}

		t.Logf("lastId: %s", lastId)
		t.Logf("ids: %v", ids)

		events, err := testChatMessageStreamCache.PullEventStream(
			ctx,
			testKey,
			schema.PullEventStreamArgs{
				LastId: lastId,
			},
		)
		So(err, ShouldBeNil)
		So(events, ShouldHaveLength, 2)

		for idx, event := range events {
			So(string(event.Data), ShouldEqual, "test-data-"+strconv.Itoa(idx+3))
		}
	})
}

func TestChatMessageStreamCache_PullEventStream_WithBlock(t *testing.T) {
	Convey("ChatMessageStreamCache PullEventStream with Block", t, func() {
		ctx := t.Context()
		testKey := "test-stream-" + uuid.NewV4().String()
		defer testChatMessageStreamCache.DeleteEventStream(ctx, testKey)

		count := 5

		var wg sync.WaitGroup
		wg.Go(func() {
			time.Sleep(100 * time.Millisecond)
			for idx := range count {
				_, err := testChatMessageStreamCache.AppendEventStream(
					ctx,
					testKey,
					&schema.ChatMessageStreamEvent{
						Data: []byte("test-data-" + strconv.Itoa(idx)),
					},
				)
				if err != nil {
					panic(err)
				}
			}
		})

		lastRecvId := ""
		idx := 0
		fetched := make([]*schema.ChatMessageStreamEvent, 0)
		var pullErr error
		for {
			idx++
			if idx == 2 {
				time.Sleep(50 * time.Millisecond)
			}
			events, err := testChatMessageStreamCache.PullEventStream(
				ctx, testKey, schema.PullEventStreamArgs{
					LastId: lastRecvId,
					Block:  1 * time.Second,
				},
			)
			if err != nil {
				// 无数据是流读完的正常终止信号，其余错误留给循环外断言，
				// 避免在轮询过程中中断 Convey。
				if !errors.Is(err, cacheerrors.ErrStreamNoData) {
					pullErr = err
					break
				}
				break
			}

			t.Logf("events length: %d", len(events))
			for _, event := range events {
				t.Logf("event: %s, data: %s", event.Id, string(event.Data))
			}
			if len(events) == 0 {
				break
			}
			lastRecvId = events[len(events)-1].Id
			fetched = append(fetched, events...)
		}

		wg.Wait()

		So(pullErr, ShouldBeNil)
		So(fetched, ShouldHaveLength, count)
		for idx, event := range fetched {
			So(string(event.Data), ShouldEqual, "test-data-"+strconv.Itoa(idx))
		}
	})
}

func TestChatMessageStreamCache_PullEventStream_WithExplicitCustomId(t *testing.T) {
	Convey("ChatMessageStreamCache PullEventStream with explicit custom id", t, func() {
		ctx := t.Context()
		testKey := "test-stream-custom-id-" + uuid.NewV4().String()
		defer testChatMessageStreamCache.DeleteEventStream(ctx, testKey)

		customIds := []string{"1000-0", "1000-1", "1000-2"}
		for idx, customId := range customIds {
			returnedId, err := testChatMessageStreamCache.AppendEventStream(
				ctx,
				testKey,
				&schema.ChatMessageStreamEvent{
					Id:   customId,
					Data: []byte("test-data-" + strconv.Itoa(idx)),
				},
			)
			So(err, ShouldBeNil)
			So(returnedId, ShouldEqual, customId)
		}

		lastRecvId := ""
		fetched := make([]*schema.ChatMessageStreamEvent, 0)
		for round := 0; round < 3; round++ {
			events, err := testChatMessageStreamCache.PullEventStream(
				ctx, testKey, schema.PullEventStreamArgs{
					LastId: lastRecvId,
					Count:  1,
				},
			)
			So(err, ShouldBeNil)
			if len(events) == 0 {
				break
			}
			So(events, ShouldHaveLength, 1)
			lastRecvId = events[0].Id
			fetched = append(fetched, events...)
		}

		So(fetched, ShouldHaveLength, len(customIds))
		for idx, event := range fetched {
			So(event.Id, ShouldEqual, customIds[idx])
			So(string(event.Data), ShouldEqual, "test-data-"+strconv.Itoa(idx))
		}
	})
}

func TestChatMessageStreamCache_SetEventStreamTTL(t *testing.T) {
	Convey("ChatMessageStreamCache SetEventStreamTTL", t, func() {
		ctx := t.Context()
		testKey := "test-stream-ttl-" + uuid.NewV4().String()
		defer testChatMessageStreamCache.DeleteEventStream(ctx, testKey)

		_, err := testChatMessageStreamCache.AppendEventStream(
			ctx,
			testKey,
			&schema.ChatMessageStreamEvent{Data: []byte("ttl-data")},
		)
		So(err, ShouldBeNil)

		err = testChatMessageStreamCache.SetEventStreamTTL(ctx, testKey, time.Minute)
		So(err, ShouldBeNil)
	})
}

func TestChatMessageStreamCache_AppendEventStream_NilGuards(t *testing.T) {
	Convey("ChatMessageStreamCache AppendEventStream nil guards", t, func() {
		ctx := t.Context()
		testKey := "test-stream-nil-" + uuid.NewV4().String()

		_, err := testChatMessageStreamCache.AppendEventStream(ctx, testKey, nil)
		So(err, ShouldNotBeNil)

		_, err = testChatMessageStreamCache.AppendEventStream(
			ctx,
			testKey,
			&schema.ChatMessageStreamEvent{Data: nil},
		)
		So(err, ShouldNotBeNil)
	})
}

func TestChatMessageStreamCache_CancelByContext(t *testing.T) {
	Convey("ChatMessageStreamCache PullEventStream respects context cancellation", t, func() {
		testKey := "test-stream-cancel-" + uuid.NewV4().String()
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(50*time.Millisecond))
		defer cancel()

		events, err := testChatMessageStreamCache.PullEventStream(
			ctx, testKey, schema.PullEventStreamArgs{Block: 2 * time.Second},
		)
		So(err, ShouldNotBeNil)
		So(errors.Is(err, context.DeadlineExceeded), ShouldBeTrue)
		So(events, ShouldHaveLength, 0)
	})
}

// task 数据被写坏（非 msgpack）时，GetTask 必须报 ErrSerde 而不是返回半截数据。
func TestChatMessageStreamCache_GetTask_CorruptedPayload(t *testing.T) {
	Convey("ChatMessageStreamCache GetTask rejects corrupted payload", t, func() {
		ctx := t.Context()
		taskId := "corrupted-task-" + uuid.NewV4().String()
		taskKey := testChatMessageStreamCacheImpl().streamTaskCacheKey(taskId)
		defer testRedis.Del(ctx, taskKey)

		err := testRedis.Set(ctx, taskKey, []byte{0xc1}, time.Minute).Err()
		So(err, ShouldBeNil)

		_, err = testChatMessageStreamCache.GetTask(ctx, taskId)
		So(errors.Is(err, errors.ErrSerde), ShouldBeTrue)
	})
}

// user/chat 映射存在但指向空 taskId 时，视作 task 不存在。
func TestChatMessageStreamCache_GetTaskByUserAndChatId_EmptyTaskId(t *testing.T) {
	Convey("ChatMessageStreamCache GetTaskByUserAndChatId with empty task id", t, func() {
		ctx := t.Context()
		userId := "user-" + uuid.NewV4().String()
		chatId := "chat-" + uuid.NewV4().String()
		mappingKey := testChatMessageStreamCacheImpl().streamTaskUserChatIdCacheKey(userId, chatId)
		defer testRedis.Del(ctx, mappingKey)

		err := testRedis.Set(ctx, mappingKey, "", time.Minute).Err()
		So(err, ShouldBeNil)

		_, err = testChatMessageStreamCache.GetTaskByUserAndChatId(ctx, userId, chatId)
		So(errors.Is(err, cacheerrors.ErrTaskNotFound), ShouldBeTrue)
	})
}

// GetTaskByUserAndChatId 拿到 taskId 后仍走 GetTask，task 记录缺失时同样是找不到。
func TestChatMessageStreamCache_GetTaskByUserAndChatId_DanglingTaskId(t *testing.T) {
	Convey("ChatMessageStreamCache GetTaskByUserAndChatId with dangling task id", t, func() {
		ctx := t.Context()
		userId := "user-" + uuid.NewV4().String()
		chatId := "chat-" + uuid.NewV4().String()
		mappingKey := testChatMessageStreamCacheImpl().streamTaskUserChatIdCacheKey(userId, chatId)
		defer testRedis.Del(ctx, mappingKey)

		err := testRedis.Set(ctx, mappingKey, "dangling-"+uuid.NewV4().String(), time.Minute).Err()
		So(err, ShouldBeNil)

		_, err = testChatMessageStreamCache.GetTaskByUserAndChatId(ctx, userId, chatId)
		So(errors.Is(err, cacheerrors.ErrTaskNotFound), ShouldBeTrue)
	})
}

// DeleteTask 会校验 payload 里的 id 与 key 是否一致，防止删错记录。
func TestChatMessageStreamCache_DeleteTask_IdMismatch(t *testing.T) {
	Convey("ChatMessageStreamCache DeleteTask rejects id mismatch", t, func() {
		ctx := t.Context()
		impl := testChatMessageStreamCacheImpl()

		task := newTestTask("user-"+uuid.NewV4().String(), "chat-"+uuid.NewV4().String())
		taskId, err := testChatMessageStreamCache.SetTask(ctx, task)
		So(err, ShouldBeNil)
		defer testChatMessageStreamCache.DeleteTask(ctx, taskId)

		// 把 task payload 复制到另一个 key 下，使 key 与 payload 内的 id 不一致。
		wrongTaskId := "moved-" + uuid.NewV4().String()
		wrongKey := impl.streamTaskCacheKey(wrongTaskId)
		defer testRedis.Del(ctx, wrongKey)

		payload, err := testRedis.Get(ctx, impl.streamTaskCacheKey(taskId)).Result()
		So(err, ShouldBeNil)

		err = testRedis.Set(ctx, wrongKey, payload, time.Minute).Err()
		So(err, ShouldBeNil)

		err = testChatMessageStreamCache.DeleteTask(ctx, wrongTaskId)
		So(errors.Is(err, errors.ErrCache), ShouldBeTrue)
	})
}

// task 记录被写坏时 DeleteTask 报 ErrSerde。
func TestChatMessageStreamCache_DeleteTask_CorruptedPayload(t *testing.T) {
	Convey("ChatMessageStreamCache DeleteTask rejects corrupted payload", t, func() {
		ctx := t.Context()
		taskId := "corrupted-task-" + uuid.NewV4().String()
		taskKey := testChatMessageStreamCacheImpl().streamTaskCacheKey(taskId)
		defer testRedis.Del(ctx, taskKey)

		err := testRedis.Set(ctx, taskKey, []byte{0xc1}, time.Minute).Err()
		So(err, ShouldBeNil)

		err = testChatMessageStreamCache.DeleteTask(ctx, taskId)
		So(errors.Is(err, errors.ErrSerde), ShouldBeTrue)
	})
}

// DeleteTask 正常路径要把 user/chat 映射一并清掉。
func TestChatMessageStreamCache_DeleteTask_CleansUserChatMapping(t *testing.T) {
	Convey("ChatMessageStreamCache DeleteTask cleans user/chat mapping", t, func() {
		ctx := t.Context()
		task := newTestTask("user-"+uuid.NewV4().String(), "chat-"+uuid.NewV4().String())

		taskId, err := testChatMessageStreamCache.SetTask(ctx, task)
		So(err, ShouldBeNil)

		err = testChatMessageStreamCache.DeleteTask(ctx, taskId)
		So(err, ShouldBeNil)

		// 映射已删除，再按 user/chat 查应当找不到。
		_, err = testChatMessageStreamCache.GetTaskByUserAndChatId(ctx, task.UserId, task.ChatId)
		So(errors.Is(err, cacheerrors.ErrTaskNotFound), ShouldBeTrue)

		mappingKey := testChatMessageStreamCacheImpl().streamTaskUserChatIdCacheKey(task.UserId, task.ChatId)
		exists, err := testRedis.Exists(ctx, mappingKey).Result()
		So(err, ShouldBeNil)
		So(exists, ShouldEqual, 0)
	})
}

// Stream 里的消息 payload 被写坏时，跳过该条并返回其余可解析事件。
func TestChatMessageStreamCache_PullEventStream_SkipsCorruptedEvent(t *testing.T) {
	Convey("ChatMessageStreamCache PullEventStream skips corrupted event", t, func() {
		ctx := t.Context()
		testKey := "test-stream-corrupted-" + uuid.NewV4().String()
		defer testChatMessageStreamCache.DeleteEventStream(ctx, testKey)

		goodId, err := testChatMessageStreamCache.AppendEventStream(
			ctx, testKey, &schema.ChatMessageStreamEvent{Data: []byte("good-data")},
		)
		So(err, ShouldBeNil)

		// 绕过 msgpack 编码，直接往 stream 里塞一条无法解码的 payload。
		badId, err := testRedis.XAdd(ctx, &goredis.XAddArgs{
			Stream: testChatMessageStreamCacheImpl().streamTaskEventCacheKey(testKey),
			Values: map[string]any{streamEventDataKey: []byte{0xc1}},
		}).Result()
		So(err, ShouldBeNil)
		So(badId > goodId, ShouldBeTrue)

		events, err := testChatMessageStreamCache.PullEventStream(ctx, testKey, schema.PullEventStreamArgs{})
		So(err, ShouldBeNil)
		So(events, ShouldHaveLength, 1)
		So(string(events[0].Data), ShouldEqual, "good-data")
		So(events[0].Id, ShouldEqual, goodId)
	})
}

// DeleteEventStream / SetEventStreamTTL 对不存在的 key 也是幂等的。
func TestChatMessageStreamCache_EventStreamOpsOnMissingKey(t *testing.T) {
	Convey("ChatMessageStreamCache event stream ops on missing key", t, func() {
		ctx := t.Context()
		testKey := "test-stream-missing-" + uuid.NewV4().String()

		err := testChatMessageStreamCache.DeleteEventStream(ctx, testKey)
		So(err, ShouldBeNil)

		err = testChatMessageStreamCache.SetEventStreamTTL(ctx, testKey, time.Minute)
		So(err, ShouldBeNil)
	})
}

// 带 Block 但流里已有数据时，应当立刻返回而不是等到超时。
func TestChatMessageStreamCache_PullEventStream_BlockWithExistingData(t *testing.T) {
	Convey("ChatMessageStreamCache PullEventStream with Block and existing data", t, func() {
		ctx := t.Context()
		testKey := "test-stream-block-ready-" + uuid.NewV4().String()
		defer testChatMessageStreamCache.DeleteEventStream(ctx, testKey)

		id, err := testChatMessageStreamCache.AppendEventStream(
			ctx, testKey, &schema.ChatMessageStreamEvent{Data: []byte("ready")},
		)
		So(err, ShouldBeNil)

		events, err := testChatMessageStreamCache.PullEventStream(ctx, testKey, schema.PullEventStreamArgs{
			Block: 2 * time.Second,
		})
		So(err, ShouldBeNil)
		So(events, ShouldHaveLength, 1)
		So(events[0].Id, ShouldEqual, id)
	})
}

// Count 限制单次拉取条数。
func TestChatMessageStreamCache_PullEventStream_WithCount(t *testing.T) {
	Convey("ChatMessageStreamCache PullEventStream with Count", t, func() {
		ctx := t.Context()
		testKey := "test-stream-count-" + uuid.NewV4().String()
		defer testChatMessageStreamCache.DeleteEventStream(ctx, testKey)

		for idx := range 3 {
			_, err := testChatMessageStreamCache.AppendEventStream(
				ctx, testKey, &schema.ChatMessageStreamEvent{Data: []byte("data-" + strconv.Itoa(idx))},
			)
			So(err, ShouldBeNil)
		}

		events, err := testChatMessageStreamCache.PullEventStream(ctx, testKey, schema.PullEventStreamArgs{Count: 2})
		So(err, ShouldBeNil)
		So(events, ShouldHaveLength, 2)
	})
}
