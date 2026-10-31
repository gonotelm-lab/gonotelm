package schema

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Favourite struct {
	Id        uuid.UUID  `gorm:"column:id;primaryKey"`
	UserId    valobj.Uid `gorm:"column:user_id"`
	Name      string     `gorm:"column:name"`
	CreatedAt int64      `gorm:"column:created_at"`
	UpdatedAt int64      `gorm:"column:updated_at"`
}

func (Favourite) TableName() string {
	return "favourites"
}
