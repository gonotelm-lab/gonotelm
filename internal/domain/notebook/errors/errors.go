package errors

import (
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const (
	CodeNotebookNotFound    = 100001
	CodeInvalidName         = 100002
	CodeInvalidDescription  = 100003
	CodeInvalidOwnerId      = 100004
	CodeSourceCountExceeded = 100005
)

var (
	ErrNotebookNotFound = errors.ErrNoRecord.ErrCode(CodeNotebookNotFound).Msg("notebook not found")

	ErrInvalidName         = errors.ErrParams.ErrCode(CodeInvalidName).Msg("invalid name")
	ErrInvalidDescription  = errors.ErrParams.ErrCode(CodeInvalidDescription).Msg("invalid description")
	ErrInvalidOwnerId      = errors.ErrParams.ErrCode(CodeInvalidOwnerId).Msg("invalid owner id")
	ErrSourceCountExceeded = errors.ErrParams.ErrCode(CodeSourceCountExceeded).Msg("source count exceeded")
)
