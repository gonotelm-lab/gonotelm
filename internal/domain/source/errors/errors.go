package source

import "github.com/gonotelm-lab/gonotelm/pkg/errors"

var (
	ErrSourceNotFound       = errors.ErrNoRecord.Msg("source not found")
	ErrSourceContentTooLong = errors.ErrParams.Msg("source content too long")
)
