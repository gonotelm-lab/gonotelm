package impl

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func TestChatMessageStreamCache_PullEventStream(t *testing.T) {
	testKey := "test-stream-key"
	defer testChatMessageStreamCache.DeleteEventStream(t.Context(), testKey)

	count := 5
	for idx := range count {
		testChatMessageStreamCache.AppendEventStream(
			t.Context(),
			"test-stream-key",
			&schema.ChatMessageStreamEvent{
				Data: []byte("test-data-" + strconv.Itoa(idx)),
			})
	}

	// read
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

	// check stream data
	for idx, event := range events {
		if string(event.Data) != "test-data-"+strconv.Itoa(idx) {
			t.Fatalf("expected test-data-%d, got %s", idx, string(event.Data))
		}
	}
}

func TestChatMessageStreamCache_PullEventStream_WithLastId(t *testing.T) {
	testKey := "test-stream-key"
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

	// check stream data
	for idx, event := range events {
		if string(event.Data) != "test-data-"+strconv.Itoa(idx+3) {
			t.Fatalf("expected test-data-%d, got %s", idx+3, string(event.Data))
		}
	}
}

func TestChatMessageStreamCache_PullEventStream_WithBlock(t *testing.T) {
	testKey := "test-stream-key"
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
			if errors.Is(err, cache.ErrStreamNoData) {
				break
			}

			t.Fatal(err)
		}

		t.Logf("events length: %d", len(events))
		for _, event := range events {
			t.Logf("event: %s, data: %s", event.StreamId(), string(event.Data))
		}
		if len(events) == 0 {
			break
		}
		lastRecvId = events[len(events)-1].StreamId()
		fetched = append(fetched, events...)
	}

	wg.Wait()

	// check fetched events
	for idx, event := range fetched {
		if string(event.Data) != "test-data-"+strconv.Itoa(idx) {
			t.Fatalf("expected test-data-%d, got %s", idx, string(event.Data))
		}
	}
}

func TestChatMessageStreamCache_CancelByContext(t *testing.T) {
	testKey := "test-stream-key"
	// ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	// defer cancel()
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	events, err := testChatMessageStreamCache.PullEventStream(
		ctx, testKey, schema.PullEventStreamArgs{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}
