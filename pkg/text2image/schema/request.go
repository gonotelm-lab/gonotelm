package schema

type Request struct {
	Model  string
	Prompt string
	Size   string // W*H

	// 仅在模型支持时有效
	ResponseFormat ResponseFormat
}
