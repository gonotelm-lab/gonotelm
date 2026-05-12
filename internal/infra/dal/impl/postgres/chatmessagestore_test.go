package postgres

import (
	"bytes"
	"encoding/json"
	"reflect"
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
			MsgRole: int8(0),
			MsgType: int8(0),
			Content: json.RawMessage(`{"text":"hello"}`),
			SeqNo:   1000,
		}
		msgNew := &schema.ChatMessage{
			ChatId:  chatID,
			UserId:  userID,
			MsgRole: int8(1),
			MsgType: int8(0),
			Content: json.RawMessage(`{"text":"world"}`),
			SeqNo:   2000,
		}

		So(store.Create(ctx, msgOld), ShouldBeNil)
		So(store.Create(ctx, msgNew), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM chat_messages WHERE chat_id = ?`, chatID).Error
		})

		gotByID, err := store.GetById(ctx, msgNew.Id)
		So(err, ShouldBeNil)
		So(gotByID.Id, ShouldEqual, msgNew.Id)
		So(gotByID.MsgRole, ShouldEqual, msgNew.MsgRole)
		So(gotByID.MsgType, ShouldEqual, msgNew.MsgType)
		So(gotByID.SeqNo, ShouldEqual, msgNew.SeqNo)

		gotByIDAndChatID, err := store.GetByIdAndChatId(ctx, msgNew.Id, chatID)
		So(err, ShouldBeNil)
		So(gotByIDAndChatID.Id, ShouldEqual, msgNew.Id)
		So(gotByIDAndChatID.ChatId, ShouldEqual, chatID)

		listed, err := store.ListByChatId(ctx, chatID, 10, 0)
		So(err, ShouldBeNil)
		So(len(listed), ShouldEqual, 2)
		So(listed[0].SeqNo, ShouldEqual, msgNew.SeqNo)
		So(listed[0].MsgRole, ShouldEqual, msgNew.MsgRole)
		So(listed[0].MsgType, ShouldEqual, msgNew.MsgType)
		So(compactJSON(listed[0].Content), ShouldEqual, compactJSON(msgNew.Content))
		So(listed[1].SeqNo, ShouldEqual, msgOld.SeqNo)
		So(listed[1].MsgRole, ShouldEqual, msgOld.MsgRole)
		So(listed[1].MsgType, ShouldEqual, msgOld.MsgType)
		So(compactJSON(listed[1].Content), ShouldEqual, compactJSON(msgOld.Content))

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

func jsonEqual(left, right json.RawMessage) bool {
	var leftVal any
	var rightVal any
	if err := json.Unmarshal(left, &leftVal); err != nil {
		return false
	}
	if err := json.Unmarshal(right, &rightVal); err != nil {
		return false
	}

	return reflect.DeepEqual(leftVal, rightVal)
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
			MsgRole: int8(0),
			MsgType: int8(0),
			Content: json.RawMessage(`{"text":"old"}`),
			SeqNo:   1000,
		}
		msgNew := &schema.ChatMessage{
			ChatId:  chatID,
			UserId:  userID,
			MsgRole: int8(1),
			MsgType: int8(0),
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
		So(firstPage[0].SeqNo, ShouldEqual, msgNew.SeqNo)
		So(firstPage[0].MsgRole, ShouldEqual, msgNew.MsgRole)
		So(firstPage[0].MsgType, ShouldEqual, msgNew.MsgType)

		secondPage, err := store.ListByChatId(ctx, chatID, 1, 1)
		So(err, ShouldBeNil)
		So(len(secondPage), ShouldEqual, 1)
		So(secondPage[0].SeqNo, ShouldEqual, msgOld.SeqNo)
		So(secondPage[0].MsgRole, ShouldEqual, msgOld.MsgRole)
		So(secondPage[0].MsgType, ShouldEqual, msgOld.MsgType)
	})
}

func TestChatMessageStoreListByChatIdBeforeSeqNoIncludesExtra(t *testing.T) {
	Convey("ChatMessageStore list by chat id before seq no includes extra", t, func() {
		store := testChatMessageStore
		ctx := t.Context()
		chatID := createNotebookForSourceTest(t, testDB)
		userID := "user_" + uuid.NewV7().String()

		msgOld := &schema.ChatMessage{
			ChatId:  chatID,
			UserId:  userID,
			MsgRole: int8(0),
			MsgType: int8(0),
			Content: json.RawMessage(`{"text":"old"}`),
			SeqNo:   1000,
		}
		msgNew := &schema.ChatMessage{
			ChatId:  chatID,
			UserId:  userID,
			MsgRole: int8(1),
			MsgType: int8(0),
			Content: json.RawMessage(`{"text":"new"}`),
			SeqNo:   2000,
			Extra:   json.RawMessage(`{"citation":{"citations":[{"source_id":"source-1","doc_ids":["doc-1"]}]}}`),
		}

		So(store.Create(ctx, msgOld), ShouldBeNil)
		So(store.Create(ctx, msgNew), ShouldBeNil)
		t.Cleanup(func() {
			_ = testDB.WithContext(ctx).Exec(`DELETE FROM chat_messages WHERE chat_id = ?`, chatID).Error
		})

		rows, err := store.ListByChatIdBeforeSeqNo(ctx, chatID, 3000, 10)
		So(err, ShouldBeNil)
		So(len(rows), ShouldEqual, 2)

		var newRow *schema.ChatMessage
		for _, row := range rows {
			if row.Id == msgNew.Id {
				newRow = row
				break
			}
		}
		So(newRow, ShouldNotBeNil)
		So(newRow.SeqNo, ShouldEqual, msgNew.SeqNo)
		So(jsonEqual(newRow.Extra, msgNew.Extra), ShouldBeTrue)
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

func TestChatMessageStoreGetByIdNotExist(t *testing.T) {
	Convey("ChatMessageStore get by id not exist", t, func() {
		store := testChatMessageStore
		ctx := t.Context()
		_, err := store.GetById(ctx, uuid.NewV7())
		So(err, ShouldNotBeNil)
	})
}

func TestChatMessageStoreGetByIdAndChatIdNotExist(t *testing.T) {
	Convey("ChatMessageStore get by id and chat id not exist", t, func() {
		store := testChatMessageStore
		ctx := t.Context()
		_, err := store.GetByIdAndChatId(ctx, uuid.NewV7(), uuid.NewV7())
		So(err, ShouldNotBeNil)
	})
}
