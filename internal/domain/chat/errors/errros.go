package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

var (
	ErrChatNotFound       = errors.ErrNoRecord.Msg("chat not found")
	ErrMessageNotFound    = errors.ErrNoRecord.Msg("message not found")
	ErrStreamTaskNotFound = errors.ErrNoRecord.Msg("stream task not found")
)
