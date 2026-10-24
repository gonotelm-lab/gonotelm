package entity

import (
	"fmt"
	"unicode/utf8"

	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
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
	Visibility  vo.Visibility

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
		Visibility:  vo.VisibilityRestricted,
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

	n.Visibility = vo.NormalizeVisibility(n.Visibility)
	if !n.Visibility.IsValid() {
		return notebookerrors.ErrInvalidVisibility
	}

	return nil
}

func (n *Notebook) SetVisibility(visibility vo.Visibility) error {
	if !visibility.IsValid() {
		return notebookerrors.ErrInvalidVisibility
	}

	n.Visibility = visibility
	n.UpdateTime = valobj.NewTime()
	return nil
}

func (n *Notebook) IsPublic() bool {
	return n.Visibility.IsPublic()
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

func (n *Notebook) UpdateDescription(description string) error {
	n.Description = description

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
