package schema

import (
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type FavouriteNotebook struct {
	FavouriteId  uuid.UUID `gorm:"column:favourite_id;primaryKey"`
	NotebookId   uuid.UUID `gorm:"column:notebook_id;primaryKey"`
	FavouritedAt int64     `gorm:"column:favourited_at"`
}

func (FavouriteNotebook) TableName() string {
	return "favourite_notebooks"
}
