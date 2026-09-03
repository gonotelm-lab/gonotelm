package qwen

import "github.com/gonotelm-lab/gonotelm/pkg/rerank"

const (
	extraKeyInstruct = "qwen_instruct"
	extraKeyFPS      = "qwen_fps"

	paramTopN            = "top_n"
	paramReturnDocuments = "return_documents"
	paramInstruct        = "instruct"
	paramFPS             = "fps"

	fieldText  = "text"
	fieldImage = "image"
	fieldVideo = "video"

	respFieldResults = "results"
	respFieldUsage   = "usage"
	respFieldModel   = "model"
	respFieldID      = "id"
)

// WithInstruct 设置 Qwen 特有的 instruct 参数。
func WithInstruct(instruct string) rerank.Option {
	return rerank.WithExtra(extraKeyInstruct, instruct)
}

// WithFPS 设置 Qwen 视频场景的帧采样率。
func WithFPS(fps float64) rerank.Option {
	if fps <= 0 {
		return nil
	}
	return rerank.WithExtra(extraKeyFPS, fps)
}
