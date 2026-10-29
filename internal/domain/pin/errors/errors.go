package errors

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodePinLimitExceeded  = 107001
	CodeInvalidNotebookId = 107002
	CodeInvalidUserId     = 107003
	CodePinNotFound       = 107004
)

var (
	ErrPinLimitExceeded  = errors.ErrParams.ErrCode(CodePinLimitExceeded).Msg("pin limit exceeded")
	ErrInvalidNotebookId = errors.ErrParams.ErrCode(CodeInvalidNotebookId).Msg("invalid notebook id")
	ErrInvalidUserId     = errors.ErrParams.ErrCode(CodeInvalidUserId).Msg("invalid user id")
	ErrPinNotFound       = errors.ErrNoRecord.ErrCode(CodePinNotFound).Msg("pin not found")
)
