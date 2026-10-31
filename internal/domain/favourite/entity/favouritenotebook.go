package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/errors"
)

const (
	MaxNotebooksPerFavourite   = 500
	MaxRemoveNotebookIdsPerReq = 100
)

type FavouriteNotebook struct {
	FavouriteId  valobj.Id
	NotebookId   valobj.Id
	FavouritedAt valobj.Time
}

func NewFavouriteNotebook(favouriteId, notebookId valobj.Id) (*FavouriteNotebook, error) {
	row := &FavouriteNotebook{
		FavouriteId:  favouriteId,
		NotebookId:   notebookId,
		FavouritedAt: valobj.NewTime(),
	}
	if err := row.validate(); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *FavouriteNotebook) validate() error {
	if r.FavouriteId.IsZero() {
		return errors.ErrInvalidFavouriteId
	}
	if r.NotebookId.IsZero() {
		return errors.ErrInvalidNotebookId
	}
	return nil
}
