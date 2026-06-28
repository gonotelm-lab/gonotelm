package notebook

import (
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

var (
	ErrNotebookNotFound = errors.ErrNoRecord.Msg("notebook not found")

	ErrInvalidName        = errors.ErrParams.Msg("invalid name")
	ErrInvalidDescription = errors.ErrParams.Msg("invalid description")
	ErrInvalidOwnerId     = errors.ErrParams.Msg("invalid owner id")

	ErrSourceCountExceeded = errors.ErrParams.Msg("source count exceeded")
)
