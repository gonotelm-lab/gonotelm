package chat

import (
	"encoding/json"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestMessageV2_MarshalJSON(t *testing.T) {
	msg := &MessageV2{
		Id:     uuid.NewV7(),
		ChatId: uuid.NewV7(),
		UserId: "user_123",
		Role:   MessageRoleAssistant,
		SeqNo:  1,
		Fragments: []*Fragment{
			{
				Id:     1,
				Type:   FragmentTypeReasoning,
				Status: FragmentStatusDone,
				Content: &FragmentContent{
					Type: ContentTypeText,
					Text: &ContentText{
						Data: "Hello world",
					},
				},
			},
			{
				Id:     2,
				Type:   FragmentTypeReasoning,
				Status: FragmentStatusWIP,
				Content: &FragmentContent{
					Type: ContentTypeImage,
					Image: &ContentImage{
						DataUrl: "data:base64:a12355",
					},
				},
			},
		},
	}

	s, _ := json.MarshalIndent(msg, "", " ")
	println(string(s))
}
