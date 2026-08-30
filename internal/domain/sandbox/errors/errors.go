package sandbox

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodeSandboxNotFound = 105001
)

var ErrSandboxNotFound = errors.ErrNoRecord.ErrCode(CodeSandboxNotFound).Msg("sandbox not found")
