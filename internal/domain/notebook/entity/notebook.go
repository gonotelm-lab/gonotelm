package entity

import (
	"fmt"
	"strings"
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

	Name         string
	Description  string // 用户自定义摘要
	Flags        vo.Flags
	OwnerId      valobj.Uid
	Visibility   vo.Visibility
	Cover        valobj.StoreKey // 生效封面
	PendingCover valobj.StoreKey // 在途 key（uploads 下发，confirm 单写点转化）

	SourceCount int64
}

func NewNotebook(
	name string,
	ownerId valobj.Uid,
) (*Notebook, error) {
	n := Notebook{
		Base:        coreentity.NewBase(),
		Name:        name,
		Description: "",
		Flags:       0,
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

func (n *Notebook) CustomizeDescription() bool {
	return n.Flags.CustomizeDescription()
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

func (n *Notebook) SetCustomizeDescription(description string) error {
	description = strings.TrimSpace(description)
	if description == "" {
		return notebookerrors.ErrInvalidDescription
	}

	n.Description = description
	n.Flags = n.Flags.WithCustomizeDescription(true)

	if err := n.validate(); err != nil {
		return err
	}

	n.UpdateTime = valobj.NewTime()
	return nil
}

func (n *Notebook) ClearCustomizeDescription() {
	n.Description = ""
	n.Flags = n.Flags.WithCustomizeDescription(false)
	n.UpdateTime = valobj.NewTime()
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
