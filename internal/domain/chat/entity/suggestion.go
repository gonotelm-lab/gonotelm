package entity

type SuggestionType string

const (
	SuggestionTypeNone     SuggestionType = "none"
	SuggestionTypeFollowUp SuggestionType = "follow_up"
	SuggestionTypeOpener   SuggestionType = "opener"
)

type Suggestion struct {
	Type      SuggestionType
	Questions []string
}

func NewSuggestion(t SuggestionType, questions []string) *Suggestion {
	return &Suggestion{
		Type:      t,
		Questions: questions,
	}
}
