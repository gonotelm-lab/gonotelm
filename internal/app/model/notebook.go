package model

import (
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
)

type Notebook struct {
	Id          Id
	Name        string
	Description string
	OwnerId     string
	UpdatedAt   int64
}

func (n *Notebook) To() *schema.Notebook {
	return &schema.Notebook{
		Id:          n.Id,
		Name:        n.Name,
		Description: n.Description,
		OwnerId:     n.OwnerId,
		UpdatedAt:   n.UpdatedAt,
	}
}

func NewNotebookFrom(n *schema.Notebook) *Notebook {
	return &Notebook{
		Id:          n.Id,
		Name:        n.Name,
		Description: n.Description,
		OwnerId:     n.OwnerId,
		UpdatedAt:   n.UpdatedAt,
	}
}
