package redis

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChatSuggestionCacheImpl(t *testing.T) {
	Convey("ChatSuggestionCache Set/Get/Delete", t, func() {
		ctx := t.Context()
		chatId := "test" + uuid.NewV7().String()

		err := testChatSuggestionCache.Set(ctx, chatId, &schema.ChatSuggestion{
			Type:      "follow_up",
			Questions: []string{"问题1", "问题2"},
		})
		So(err, ShouldBeNil)

		got, err := testChatSuggestionCache.Get(ctx, chatId)
		So(err, ShouldBeNil)
		So(got.Type, ShouldEqual, "follow_up")
		So(got.Questions, ShouldHaveLength, 2)
		So(got.Questions[0], ShouldEqual, "问题1")
		So(got.Questions[1], ShouldEqual, "问题2")

		err = testChatSuggestionCache.Delete(ctx, chatId)
		So(err, ShouldBeNil)
	})
}

func TestChatSuggestionCacheImplNotFound(t *testing.T) {
	Convey("ChatSuggestionCache get missing suggestion", t, func() {
		chatId := "test" + uuid.NewV7().String()

		got, err := testChatSuggestionCache.Get(t.Context(), chatId)
		So(err, ShouldBeNil)
		So(got, ShouldBeNil)
	})
}
