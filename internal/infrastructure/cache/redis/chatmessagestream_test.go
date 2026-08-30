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
)

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
	task := newTestTask("user-"+uuid.NewV4().String(), "chat-"+uuid.NewV4().String())

	taskId, err := testChatMessageStreamCache.SetTask(t.Context(), task)
	if err != nil {
		t.Fatal(err)
	}
	if taskId != task.Id {
		t.Fatalf("expected task id %s, got %s", task.Id, taskId)
	}
	defer testChatMessageStreamCache.DeleteTask(t.Context(), taskId)

	got, err := testChatMessageStreamCache.GetTask(t.Context(), taskId)
	if err != nil {
		t.Fatal(err)
	}
	if got.Id != task.Id {
		t.Fatalf("expected id %s, got %s", task.Id, got.Id)
	}
	if got.UserId != task.UserId || got.ChatId != task.ChatId {
		t.Fatalf("unexpected user/chat: %+v", got)
	}
	if got.Status != task.Status {
		t.Fatalf("expected status %s, got %s", task.Status, got.Status)
	}
	if len(got.SourceIds) != 2 || got.SourceIds[0] != "src-1" {
		t.Fatalf("unexpected source ids: %v", got.SourceIds)
	}

	byUserChat, err := testChatMessageStreamCache.GetTaskByUserAndChatId(t.Context(), task.UserId, task.ChatId)
	if err != nil {
		t.Fatal(err)
	}
	if byUserChat.Id != task.Id {
		t.Fatalf("expected id %s via user/chat, got %s", task.Id, byUserChat.Id)
	}

	if err := testChatMessageStreamCache.DeleteTask(t.Context(), taskId); err != nil {
		t.Fatal(err)
	}

	_, err = testChatMessageStreamCache.GetTask(t.Context(), taskId)
	if !errors.Is(err, cacheerrors.ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound after delete, got %v", err)
	}
	_, err = testChatMessageStreamCache.GetTaskByUserAndChatId(t.Context(), task.UserId, task.ChatId)
	if !errors.Is(err, cacheerrors.ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound for user/chat after delete, got %v", err)
	}
}

func TestChatMessageStreamCache_SetTask_AutoGenerateId(t *testing.T) {
	task := newTestTask("user-"+uuid.NewV4().String(), "chat-"+uuid.NewV4().String())
	task.Id = ""

	taskId, err := testChatMessageStreamCache.SetTask(t.Context(), task)
	if err != nil {
		t.Fatal(err)
	}
	defer testChatMessageStreamCache.DeleteTask(t.Context(), taskId)

	if taskId == "" {
		t.Fatal("expected auto-generated task id")
	}
	if task.Id != taskId {
		t.Fatalf("expected task.Id mutated to %s, got %s", taskId, task.Id)
	}

	got, err := testChatMessageStreamCache.GetTask(t.Context(), taskId)
	if err != nil {
		t.Fatal(err)
	}
	if got.Id != taskId {
		t.Fatalf("expected id %s, got %s", taskId, got.Id)
	}
}

func TestChatMessageStreamCache_GetTask_NotFound(t *testing.T) {
	_, err := testChatMessageStreamCache.GetTask(t.Context(), "missing-task-"+uuid.NewV4().String())
	if !errors.Is(err, cacheerrors.ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestChatMessageStreamCache_GetTaskByUserAndChatId_NotFound(t *testing.T) {
	_, err := testChatMessageStreamCache.GetTaskByUserAndChatId(
		t.Context(),
		"missing-user-"+uuid.NewV4().String(),
		"missing-chat-"+uuid.NewV4().String(),
	)
	if !errors.Is(err, cacheerrors.ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestChatMessageStreamCache_DeleteTask_NotFound(t *testing.T) {
	err := testChatMessageStreamCache.DeleteTask(t.Context(), "missing-task-"+uuid.NewV4().String())
	if !errors.Is(err, cacheerrors.ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestChatMessageStreamCache_SetTask_OverwriteUserChatMapping(t *testing.T) {
	userId := "user-" + uuid.NewV4().String()
	chatId := "chat-" + uuid.NewV4().String()

	first := newTestTask(userId, chatId)
	firstId, err := testChatMessageStreamCache.SetTask(t.Context(), first)
	if err != nil {
		t.Fatal(err)
	}
	defer testChatMessageStreamCache.DeleteTask(t.Context(), firstId)

	second := newTestTask(userId, chatId)
	secondId, err := testChatMessageStreamCache.SetTask(t.Context(), second)
	if err != nil {
		t.Fatal(err)
	}
	defer testChatMessageStreamCache.DeleteTask(t.Context(), secondId)

	got, err := testChatMessageStreamCache.GetTaskByUserAndChatId(t.Context(), userId, chatId)
	if err != nil {
		t.Fatal(err)
	}
	if got.Id != secondId {
		t.Fatalf("expected mapping to point to latest task %s, got %s", secondId, got.Id)
	}
}

func TestChatMessageStreamCache_PullEventStream(t *testing.T) {
	testKey := "test-stream-" + uuid.NewV4().String()
	defer testChatMessageStreamCache.DeleteEventStream(t.Context(), testKey)

	count := 5
	for idx := range count {
		_, err := testChatMessageStreamCache.AppendEventStream(
			t.Context(),
			testKey,
			&schema.ChatMessageStreamEvent{
				Data: []byte("test-data-" + strconv.Itoa(idx)),
			})
		if err != nil {
			t.Fatal(err)
		}
	}

	events, err := testChatMessageStreamCache.PullEventStream(
		t.Context(),
		testKey,
		schema.PullEventStreamArgs{},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != count {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	for idx, event := range events {
		if string(event.Data) != "test-data-"+strconv.Itoa(idx) {
			t.Fatalf("expected test-data-%d, got %s", idx, string(event.Data))
		}
	}
}

func TestChatMessageStreamCache_PullEventStream_WithLastId(t *testing.T) {
	testKey := "test-stream-" + uuid.NewV4().String()
	defer testChatMessageStreamCache.DeleteEventStream(t.Context(), testKey)

	count := 5
	lastId := ""
	ids := make([]string, 0, count)
	for idx := range count {
		id, err := testChatMessageStreamCache.AppendEventStream(
			t.Context(),
			testKey,
			&schema.ChatMessageStreamEvent{
				Data: []byte("test-data-" + strconv.Itoa(idx)),
			})
		if err != nil {
			t.Fatal(err)
		}
		if idx == 2 {
			lastId = id
		}
		ids = append(ids, id)
	}

	t.Logf("lastId: %s", lastId)
	t.Logf("ids: %v", ids)

	events, err := testChatMessageStreamCache.PullEventStream(
		t.Context(),
		testKey,
		schema.PullEventStreamArgs{
			LastId: lastId,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	for idx, event := range events {
		if string(event.Data) != "test-data-"+strconv.Itoa(idx+3) {
			t.Fatalf("expected test-data-%d, got %s", idx+3, string(event.Data))
		}
	}
}

func TestChatMessageStreamCache_PullEventStream_WithBlock(t *testing.T) {
	testKey := "test-stream-" + uuid.NewV4().String()
	defer testChatMessageStreamCache.DeleteEventStream(t.Context(), testKey)

	count := 5

	var wg sync.WaitGroup
	wg.Go(func() {
		time.Sleep(100 * time.Millisecond)
		for idx := range count {
			_, err := testChatMessageStreamCache.AppendEventStream(
				t.Context(),
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
	for {
		idx++
		if idx == 2 {
			time.Sleep(50 * time.Millisecond)
		}
		events, err := testChatMessageStreamCache.PullEventStream(
			t.Context(), testKey, schema.PullEventStreamArgs{
				LastId: lastRecvId,
				Block:  1 * time.Second,
			},
		)
		if err != nil {
			if errors.Is(err, cacheerrors.ErrStreamNoData) {
				break
			}

			t.Fatal(err)
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

	for idx, event := range fetched {
		if string(event.Data) != "test-data-"+strconv.Itoa(idx) {
			t.Fatalf("expected test-data-%d, got %s", idx, string(event.Data))
		}
	}
}

func TestChatMessageStreamCache_PullEventStream_WithExplicitCustomId(t *testing.T) {
	testKey := "test-stream-custom-id-" + uuid.NewV4().String()
	defer testChatMessageStreamCache.DeleteEventStream(t.Context(), testKey)

	customIds := []string{"1000-0", "1000-1", "1000-2"}
	for idx, customId := range customIds {
		returnedId, err := testChatMessageStreamCache.AppendEventStream(
			t.Context(),
			testKey,
			&schema.ChatMessageStreamEvent{
				Id:   customId,
				Data: []byte("test-data-" + strconv.Itoa(idx)),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if returnedId != customId {
			t.Fatalf("expected returned id %s, got %s", customId, returnedId)
		}
	}

	lastRecvId := ""
	fetched := make([]*schema.ChatMessageStreamEvent, 0)
	for round := 0; round < 3; round++ {
		events, err := testChatMessageStreamCache.PullEventStream(
			t.Context(), testKey, schema.PullEventStreamArgs{
				LastId: lastRecvId,
				Count:  1,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			break
		}
		if len(events) != 1 {
			t.Fatalf("round %d: expected 1 incremental event, got %d", round, len(events))
		}
		lastRecvId = events[0].Id
		fetched = append(fetched, events...)
	}

	if len(fetched) != len(customIds) {
		t.Fatalf("expected %d events, got %d", len(customIds), len(fetched))
	}
	for idx, event := range fetched {
		if event.Id != customIds[idx] {
			t.Fatalf("expected id %s, got %s", customIds[idx], event.Id)
		}
		if string(event.Data) != "test-data-"+strconv.Itoa(idx) {
			t.Fatalf("expected test-data-%d, got %s", idx, string(event.Data))
		}
	}
}

func TestChatMessageStreamCache_SetEventStreamTTL(t *testing.T) {
	testKey := "test-stream-ttl-" + uuid.NewV4().String()
	defer testChatMessageStreamCache.DeleteEventStream(t.Context(), testKey)

	_, err := testChatMessageStreamCache.AppendEventStream(
		t.Context(),
		testKey,
		&schema.ChatMessageStreamEvent{Data: []byte("ttl-data")},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := testChatMessageStreamCache.SetEventStreamTTL(t.Context(), testKey, time.Minute); err != nil {
		t.Fatal(err)
	}
}

func TestChatMessageStreamCache_AppendEventStream_NilGuards(t *testing.T) {
	testKey := "test-stream-nil-" + uuid.NewV4().String()

	_, err := testChatMessageStreamCache.AppendEventStream(t.Context(), testKey, nil)
	if err == nil {
		t.Fatal("expected error for nil event")
	}

	_, err = testChatMessageStreamCache.AppendEventStream(
		t.Context(),
		testKey,
		&schema.ChatMessageStreamEvent{Data: nil},
	)
	if err == nil {
		t.Fatal("expected error for nil event data")
	}
}

func TestChatMessageStreamCache_CancelByContext(t *testing.T) {
	testKey := "test-stream-cancel-" + uuid.NewV4().String()
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(50*time.Millisecond))
	defer cancel()

	events, err := testChatMessageStreamCache.PullEventStream(
		ctx, testKey, schema.PullEventStreamArgs{Block: 2 * time.Second},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}
