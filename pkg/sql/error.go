package sql

import (
	"errors"

	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"gorm.io/gorm"
)

func WrapErr(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pkgerrors.WithStack(pkgerrors.ErrNoRecord)
	}

	return pkgerrors.Wrap(pkgerrors.ErrDatabase, err.Error())
}
