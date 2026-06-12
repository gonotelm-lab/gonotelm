package glm

import (
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/rerank"
)

const (
	extraKeyReturnDocuments = "glm_return_documents"
	extraKeyReturnRawScores = "glm_return_raw_scores"
	extraKeyRequestID       = "glm_request_id"
	extraKeyUserID          = "glm_user_id"
)

// WithReturnDocuments 控制是否返回原始文档。
func WithReturnDocuments(enabled bool) rerank.Option {
	return rerank.WithExtra(extraKeyReturnDocuments, enabled)
}

// WithReturnRawScores 控制是否返回原始分数。
func WithReturnRawScores(enabled bool) rerank.Option {
	return rerank.WithExtra(extraKeyReturnRawScores, enabled)
}

// WithRequestID 设置请求追踪 ID。
func WithRequestID(requestID string) rerank.Option {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil
	}
	return rerank.WithExtra(extraKeyRequestID, requestID)
}

// WithUserID 设置终端用户 ID。
func WithUserID(userID string) rerank.Option {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	return rerank.WithExtra(extraKeyUserID, userID)
}
