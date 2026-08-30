package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodeChatNotFound         = 101001
	CodeMessageNotFound      = 101002
	CodeStreamTaskNotFound   = 101003
	CodeSuggestionGenerating = 101004
)

var (
	ErrChatNotFound       = errors.ErrNoRecord.ErrCode(CodeChatNotFound).Msg("chat not found")
	ErrMessageNotFound    = errors.ErrNoRecord.ErrCode(CodeMessageNotFound).Msg("message not found")
	ErrStreamTaskNotFound = errors.ErrNoRecord.ErrCode(CodeStreamTaskNotFound).Msg("stream task not found")

	ErrSuggestionGenerating = errors.ErrLockTaken.ErrCode(CodeSuggestionGenerating).Msg("suggestion is generating")
)
