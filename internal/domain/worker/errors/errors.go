package worker

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

var (
	ErrCheckpointNotFound = errors.ErrNoRecord.Msg("checkpoint not found")
)
