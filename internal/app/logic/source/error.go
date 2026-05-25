package source

import (
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func wrapGetSourceError(err error, sourceId uuid.UUID) error {
	if errors.Is(err, bizsource.ErrSourceNotFound) {
		return errors.ErrParams.Msgf("source not found, id=%s", sourceId)
	}

	return errors.WithStack(err)
}
