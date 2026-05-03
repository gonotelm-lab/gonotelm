package postgres

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChatMessageStoreCreateListDeleteByChatId(t *testing.T) {
	Convey("ChatMessageStore create/list/delete by chat id", t, func() {
		store := testChatMessageStore
		ctx := t.Context()
		chatID := createNotebookForSourceTest(t, testDB)
		userID := "user_" + uuid.NewV7().String()

		msgOld := &schema.ChatMessage{
			ChatId:  chatID,
			UserId:  userID,
			Role:    "user",
			Content: json.RawMessage(`{"text":"hello"}`),
			SeqNo:   1000,
		}
		msgNew := &schema.ChatMessage{
			ChatId:  chatID,
			UserId:  userID,
			Role:    "assistant",
			Content: json.RawMessage(`{"text":"world"}`),
			SeqNo:   2000,
		}

		So(store.Create(ctx, msgOld), ShouldBeNil)
		So(store.Create(ctx, msgNew), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM chat_messages WHERE chat_id = ?`, chatID).Error
		})

		listed, err := store.ListByChatId(ctx, chatID, 10, 0)
		So(err, ShouldBeNil)
		So(len(listed), ShouldEqual, 2)
		So(listed[0].SeqNo, ShouldEqual, msgOld.SeqNo)
		So(listed[0].Role, ShouldEqual, msgOld.Role)
		So(compactJSON(listed[0].Content), ShouldEqual, compactJSON(msgOld.Content))
		So(listed[1].SeqNo, ShouldEqual, msgNew.SeqNo)
		So(listed[1].Role, ShouldEqual, msgNew.Role)
		So(compactJSON(listed[1].Content), ShouldEqual, compactJSON(msgNew.Content))

		err = store.DeleteByChatId(ctx, chatID)
		So(err, ShouldBeNil)

		var count int64
		err = testDB.WithContext(ctx).Raw(
			`SELECT COUNT(1) FROM chat_messages WHERE chat_id = ?`,
			chatID,
		).Scan(&count).Error
		So(err, ShouldBeNil)
		So(count, ShouldEqual, int64(0))
	})
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

func TestChatMessageStoreListByChatIdPagination(t *testing.T) {
	Convey("ChatMessageStore list by chat id pagination", t, func() {
		store := testChatMessageStore
		ctx := t.Context()
		chatID := createNotebookForSourceTest(t, testDB)
		userID := "user_" + uuid.NewV7().String()

		msgOld := &schema.ChatMessage{
			ChatId:  chatID,
			UserId:  userID,
			Role:    "user",
			Content: json.RawMessage(`{"text":"old"}`),
			SeqNo:   1000,
		}
		msgNew := &schema.ChatMessage{
			ChatId:  chatID,
			UserId:  userID,
			Role:    "assistant",
			Content: json.RawMessage(`{"text":"new"}`),
			SeqNo:   2000,
		}

		So(store.Create(ctx, msgOld), ShouldBeNil)
		So(store.Create(ctx, msgNew), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM chat_messages WHERE chat_id = ?`, chatID).Error
		})

		firstPage, err := store.ListByChatId(ctx, chatID, 1, 0)
		So(err, ShouldBeNil)
		So(len(firstPage), ShouldEqual, 1)
		So(firstPage[0].SeqNo, ShouldEqual, msgOld.SeqNo)

		secondPage, err := store.ListByChatId(ctx, chatID, 1, 1)
		So(err, ShouldBeNil)
		So(len(secondPage), ShouldEqual, 1)
		So(secondPage[0].SeqNo, ShouldEqual, msgNew.SeqNo)
	})
}

func TestChatMessageStoreListByChatIdInvalidPagination(t *testing.T) {
	Convey("ChatMessageStore list by chat id invalid pagination", t, func() {
		store := testChatMessageStore
		ctx := t.Context()
		chatID := uuid.NewV7()

		_, err := store.ListByChatId(ctx, chatID, 0, 0)
		So(err, ShouldNotBeNil)
		So(strings.Contains(err.Error(), "invalid pagination params"), ShouldBeTrue)

		_, err = store.ListByChatId(ctx, chatID, 1, -1)
		So(err, ShouldNotBeNil)
		So(strings.Contains(err.Error(), "invalid pagination params"), ShouldBeTrue)
	})
}
