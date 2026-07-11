package vo

type AnswerLength string

const (
	AnswerLengthDefault AnswerLength = "default"
	AnswerLengthLonger  AnswerLength = "longer"
	AnswerLengthShorter AnswerLength = "shorter"
)

func (l AnswerLength) String() string {
	return string(l)
}

func (l AnswerLength) IsValid() bool {
	switch l {
	case AnswerLengthDefault, AnswerLengthLonger, AnswerLengthShorter:
		return true
	default:
		return false
	}
}
