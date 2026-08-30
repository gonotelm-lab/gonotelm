package worker

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

const (
	CodeCheckpointNotFound = 104001
)

var ErrCheckpointNotFound = errors.ErrNoRecord.ErrCode(CodeCheckpointNotFound).Msg("checkpoint not found")
