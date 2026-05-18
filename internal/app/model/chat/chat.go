package chat

import "github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"

type Chat struct {
	Id         Id
	NotebookId Id
	OwnerId    string
	UpdatedAt  int64
}

func NewChat(
	sc *schema.Chat,
) *Chat {
	return &Chat{
		Id:         Id(sc.Id),
		NotebookId: Id(sc.NotebookId),
		OwnerId:    sc.OwnerId,
		UpdatedAt:  sc.UpdatedAt,
	}
}

type ChatStyle string

const (
	ChatStyleDefault ChatStyle = "default"
	ChatStyleAnalyst ChatStyle = "analyst"
	ChatStyleGuide   ChatStyle = "guide"
)

func (s ChatStyle) String() string {
	return string(s)
}

func (s ChatStyle) IsValid() bool {
	switch s {
	case ChatStyleDefault, ChatStyleAnalyst, ChatStyleGuide:
		return true
	default:
		return false
	}
}

type ChatAnswerLength string

const (
	ChatAnswerLengthDefault ChatAnswerLength = "default"
	ChatAnswerLengthLonger  ChatAnswerLength = "longer"
	ChatAnswerLengthShorter ChatAnswerLength = "shorter"
)

func (l ChatAnswerLength) String() string {
	return string(l)
}

func (l ChatAnswerLength) IsValid() bool {
	switch l {
	case ChatAnswerLengthDefault, ChatAnswerLengthLonger, ChatAnswerLengthShorter:
		return true
	default:
		return false
	}
}
