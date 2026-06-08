package context

// 标识是用户触发的操作 还是agent触发的操作 还是其他类型
type OperatorType string

const (
	OperatorTypeUser  OperatorType = "user" // user 或者为空 表示用户触发的操作
	OperatorTypeAgent OperatorType = "agent"
)
