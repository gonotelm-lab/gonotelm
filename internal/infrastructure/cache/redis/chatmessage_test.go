package redis

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChatMessageContextCacheImpl(t *testing.T) {
	Convey("ChatMessageContextCache Append/ListAll/Destroy", t, func() {
		ctx := t.Context()
		chatId := "test" + uuid.NewV7().String()

		err := testChatMessageContextCache.Append(ctx, chatId, []*schema.ChatContextMessage{
			{
				Message: []byte("{\"name\": \"ryan\"}"),
			},
		})
		So(err, ShouldBeNil)

		listed, err := testChatMessageContextCache.ListAll(ctx, chatId)
		So(err, ShouldBeNil)
		So(listed, ShouldHaveLength, 1)
		So(string(listed[0].Message), ShouldEqual, "{\"name\": \"ryan\"}")

		err = testChatMessageContextCache.Destroy(ctx, chatId)
		So(err, ShouldBeNil)
	})
}

func TestChatMessageContextCacheImplOverride(t *testing.T) {
	Convey("ChatMessageContextCache Override", t, func() {
		ctx := t.Context()
		chatId := "test" + uuid.NewV7().String()

		err := testChatMessageContextCache.Append(ctx, chatId, []*schema.ChatContextMessage{
			{
				Message: []byte("{\"name\": \"ryan\"}"),
			},
		})
		So(err, ShouldBeNil)

		err = testChatMessageContextCache.Override(ctx, chatId, []*schema.ChatContextMessage{
			{
				Message: []byte("{\"name\": \"assistant\"}"),
			},
		})
		So(err, ShouldBeNil)

		listed, err := testChatMessageContextCache.ListAll(ctx, chatId)
		So(err, ShouldBeNil)
		So(listed, ShouldHaveLength, 1)
		So(string(listed[0].Message), ShouldEqual, "{\"name\": \"assistant\"}")

		err = testChatMessageContextCache.Destroy(ctx, chatId)
		So(err, ShouldBeNil)
	})
}

func TestChatMessageContextCacheImplList(t *testing.T) {
	Convey("ChatMessageContextCache List/ListRecent", t, func() {
		ctx := t.Context()
		chatId := "test" + uuid.NewV7().String()
		defer testChatMessageContextCache.Destroy(ctx, chatId)

		messages := make([]*schema.ChatContextMessage, 0, 5)
		for idx := range 5 {
			messages = append(messages, &schema.ChatContextMessage{
				Message: []byte(string(rune('a' + idx))),
			})
		}
		err := testChatMessageContextCache.Append(ctx, chatId, messages)
		So(err, ShouldBeNil)

		listed, err := testChatMessageContextCache.List(ctx, chatId, 1, 2)
		So(err, ShouldBeNil)
		So(listed, ShouldHaveLength, 2)
		So(string(listed[0].Message), ShouldEqual, "b")
		So(string(listed[1].Message), ShouldEqual, "c")

		recent, err := testChatMessageContextCache.ListRecent(ctx, chatId, 2)
		So(err, ShouldBeNil)
		So(recent, ShouldHaveLength, 2)
		So(string(recent[0].Message), ShouldEqual, "d")
		So(string(recent[1].Message), ShouldEqual, "e")
	})
}
