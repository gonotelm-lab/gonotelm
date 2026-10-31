package favourite

import (
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type FavouriteView struct {
	Id            valobj.Id
	Name          string
	NotebookCount int64
	CreatedAt     int64
}

type FavouriteNotebookItem struct {
	NotebookId   valobj.Id
	Invalid      bool
	FavouritedAt int64
	Notebook     *notebookapp.Notebook // 仅 Invalid=false 时有值
}
