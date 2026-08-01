package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
)

func SuggestionToSchema(s *entity.Suggestion) *schema.ChatSuggestion {
	if s == nil {
		return nil
	}

	return &schema.ChatSuggestion{
		Type:      string(s.Type),
		Questions: s.Questions,
	}
}

func SuggestionFromSchema(sch *schema.ChatSuggestion) *entity.Suggestion {
	return &entity.Suggestion{
		Type:      entity.SuggestionType(sch.Type),
		Questions: sch.Questions,
	}
}
