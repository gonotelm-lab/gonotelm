package entity

import (
	"fmt"
	"unicode/utf8"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
)

const (
	MaxNameLength        = 128
	MaxDescriptionLength = 1024
)

const (
	MaxSourceCountAllowed = 50
)

type Notebook struct {
	coreentity.Base

	Name        string
	Description string
	OwnerId     valobj.Uid

	SourceCount int64
}

func NewNotebook(
	name string,
	description string,
	ownerId valobj.Uid,
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

	n.AddEvent(notebookevent.NewCreateEvent(n.Id))

	return &n, nil
}

func (n *Notebook) validate() error {
	if len := utf8.RuneCountInString(n.Name); len > MaxNameLength {
		return notebookerrors.ErrInvalidName
	}

	if len := utf8.RuneCountInString(n.Description); len > MaxDescriptionLength {
		return notebookerrors.ErrInvalidDescription
	}

	if n.OwnerId.IsZero() {
		return notebookerrors.ErrInvalidOwnerId
	}

	return nil
}

func (n *Notebook) Delete() {
	n.Base.Delete()
	n.AddEvent(notebookevent.NewDeleteEvent(n.Id))
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

func (n *Notebook) NameAndDescription() string {
	return fmt.Sprintf("Name: %s\nDescription: %s", n.Name, n.Description)
}
