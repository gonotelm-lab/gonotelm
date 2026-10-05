package schema

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type User struct {
	Id        valobj.Uid `gorm:"column:id"`
	Email     string     `gorm:"column:email"`
	Nickname  string     `gorm:"column:nickname"`
	Status    string     `gorm:"column:status"`
	Avatar    string     `gorm:"column:avatar"`
	Provider  string     `gorm:"column:provider"`
	Sub       string     `gorm:"column:sub"`
	CreatedAt int64      `gorm:"column:created_at"`
	UpdatedAt int64      `gorm:"column:updated_at"`
}

func (User) TableName() string {
	return "users"
}
