package notebook

import (
	"unicode/utf8"

	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
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
	entity.Base

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
		Base:        entity.NewBase(),
		Name:        name,
		Description: description,
		OwnerId:     ownerId,
	}

	if err := n.validate(); err != nil {
		return nil, err
	}

	n.AddEvent(NewEvent(n.Id, EventActionCreate))

	return &n, nil
}

func (n *Notebook) validate() error {
	if len := utf8.RuneCountInString(n.Name); len > MaxNameLength {
		return ErrInvalidName
	}

	if len := utf8.RuneCountInString(n.Description); len > MaxDescriptionLength {
		return ErrInvalidDescription
	}

	if len := utf8.RuneCountInString(n.OwnerId); len > MaxOwnerIdLength {
		return ErrInvalidOwnerId
	}

	return nil
}


func (n *Notebook) Delete() {
	n.Base.Delete()
	n.Base.AddEvent(NewEvent(n.Id, EventActionDelete))
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
		return ErrSourceCountExceeded
	}

	return nil
}
