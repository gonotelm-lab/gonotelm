package mapper

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatdomain "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	schema 	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestMessageCitationRoundtrip(t *testing.T) {
	docID := valobj.Id(uuid.NewV7())
	sourceID := valobj.Id(uuid.NewV7())
	msg := chatdomain.NewAssistantMessage(valobj.Id(uuid.NewV7()), valobj.Id(uuid.NewV7()), "user1")
	msg.SetCitations([]chatdomain.MessageCitation{
		{DocId: docID, SourceId: sourceID},
	})

	sch, err := MessageToSchema(msg)
	if err != nil {
		t.Fatalf("MessageToSchema: %v", err)
	}

	loaded, err := MessageFromSchema(sch)
	if err != nil {
		t.Fatalf("MessageFromSchema: %v", err)
	}
	if len(loaded.Citations) != 1 {
		t.Fatalf("citations len=%d want 1", len(loaded.Citations))
	}
	if loaded.Citations[0].DocId != docID {
		t.Fatalf("doc_id=%v want %v", loaded.Citations[0].DocId, docID)
	}
	if loaded.Citations[0].SourceId != sourceID {
		t.Fatalf("source_id=%v want %v", loaded.Citations[0].SourceId, sourceID)
	}
}

func TestMessageFromSchemaEmptyExtra(t *testing.T) {
	sch := &schema.ChatMessage{
		Id:      uuid.NewV7(),
		ChatId:  uuid.NewV7(),
		UserId:  "u1",
		MsgRole: 1,
		Content: []byte(`[]`),
		SeqNo:   1,
	}
	loaded, err := MessageFromSchema(sch)
	if err != nil {
		t.Fatalf("MessageFromSchema nil extra: %v", err)
	}
	if len(loaded.Citations) != 0 {
		t.Fatalf("citations=%v want empty", loaded.Citations)
	}
}

func TestMessageFromSchemaNullAndEmptyExtra(t *testing.T) {
	base := &schema.ChatMessage{
		Id:      uuid.NewV7(),
		ChatId:  uuid.NewV7(),
		UserId:  "u1",
		MsgRole: 1,
		Content: []byte(`[]`),
		SeqNo:   1,
	}

	for name, extra := range map[string][]byte{
		"null":  nil,
		"empty": {},
		"obj":   []byte(`{}`),
	} {
		t.Run(name, func(t *testing.T) {
			sch := *base
			sch.Extra = extra
			loaded, err := MessageFromSchema(&sch)
			if err != nil {
				t.Fatalf("MessageFromSchema: %v", err)
			}
			if len(loaded.Citations) != 0 {
				t.Fatalf("citations=%v", loaded.Citations)
			}
		})
	}
}
