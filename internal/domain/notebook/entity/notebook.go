package entity

import (
	"unicode/utf8"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
)

const (
	MaxNameLength        = 128
	MaxDescriptionLength = 1024
	MaxOwnerIdLength     = 255
)

const (
	MaxSourceCountAllowed = 50
)

type Notebook struct {
	coreentity.Base

	Name        string
	Description string
	OwnerId     string

	SourceCount int64
}

func NewNotebook(
	name string,
	description string,
	ownerId string,
) (*Notebook, error) {
	n := Notebook{
		Base:        coreentity.NewBase(),
		Name:        name,
		Description: description,
		OwnerId:     ownerId,
	}

	if err := n.validate(); err != nil {
		return nil, err
	}

	n.AddEvent(notebookevent.NewEvent(n.Id, notebookevent.EventActionCreate))

	return &n, nil
}

func (n *Notebook) validate() error {
	if len := utf8.RuneCountInString(n.Name); len > MaxNameLength {
		return notebookerrors.ErrInvalidName
	}

	if len := utf8.RuneCountInString(n.Description); len > MaxDescriptionLength {
		return notebookerrors.ErrInvalidDescription
	}

	if len := utf8.RuneCountInString(n.OwnerId); len > MaxOwnerIdLength {
		return notebookerrors.ErrInvalidOwnerId
	}

	return nil
}

func (n *Notebook) Delete() {
	n.Base.Delete()
	n.Base.AddEvent(notebookevent.NewEvent(n.Id, notebookevent.EventActionDelete))
}

func (n *Notebook) UpdateName(name string) error {
	n.Name = name

	if err := n.validate(); err != nil {
		return err
	}

	n.UpdateTime = valobj.NewTime()
	return nil
}

func (n *Notebook) AllowedToCreateSource() error {
	if n.SourceCount >= MaxSourceCountAllowed {
		return notebookerrors.ErrSourceCountExceeded
	}

	return nil
}
