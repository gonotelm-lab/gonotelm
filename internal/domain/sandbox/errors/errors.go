package sandbox

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

var ErrSandboxNotFound = errors.ErrNoRecord.Msg("sandbox not found")
