package entity

import (
	"strings"
	"unicode/utf8"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/errors"
)

const (
	MaxFavouritesPerUser   = 100
	MaxFavouriteNameLength = 100
)

type Favourite struct {
	coreentity.Base

	UserId valobj.Uid
	Name   string
}

func NewFavourite(userId valobj.Uid, name string) (*Favourite, error) {
	f := &Favourite{
		Base:   coreentity.NewBase(),
		UserId: userId,
		Name:   strings.TrimSpace(name),
	}
	if err := f.validate(); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Favourite) Rename(name string) error {
	f.Name = strings.TrimSpace(name)
	if err := f.validate(); err != nil {
		return err
	}
	f.UpdateTime = valobj.NewTime()
	return nil
}

func (f *Favourite) validate() error {
	if f.UserId.IsZero() {
		return errors.ErrInvalidUserId
	}
	if f.Name == "" || utf8.RuneCountInString(f.Name) > MaxFavouriteNameLength {
		return errors.ErrInvalidFavouriteName
	}
	return nil
}
