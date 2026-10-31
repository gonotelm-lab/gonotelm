package notebook

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
)

type NotebookOwner struct {
	Id        valobj.Uid
	Nickname  string
	AvatarURL string
}

// FavouriteItem 当前用户将该本放入的某个收藏夹。
type FavouriteItem struct {
	Id   valobj.Id
	Name string
}

// Favourite GET 详情用的收藏嵌套视图；列表不填，靠 omitempty 省略。
// 是否已收藏由 Items 是否非空推断。
type Favourite struct {
	Items []*FavouriteItem
}

// Notebook 应用层笔记本视图，Get / List / Pin 列表共用。
type Notebook struct {
	*notebookentity.Notebook
	Owner     *NotebookOwner
	CoverUrl  string
	IsPinned  bool
	PinnedAt  int64
	Favourite *Favourite // 仅 GetNotebook 填充
}
