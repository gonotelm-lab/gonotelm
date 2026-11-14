package vo

// AnswerLength 是回答长度偏好，对应请求里的 answer_length 字段。
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
