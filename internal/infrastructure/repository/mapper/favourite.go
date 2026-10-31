package mapper

import (
	coreentity "github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func FavouriteToSchema(f *entity.Favourite) *schema.Favourite {
	return &schema.Favourite{
		Id:        f.Id,
		UserId:    f.UserId,
		Name:      f.Name,
		CreatedAt: f.CreateTime.Value(),
		UpdatedAt: f.UpdateTime.Value(),
	}
}

func FavouriteFromSchema(row *schema.Favourite) *entity.Favourite {
	return &entity.Favourite{
		Base: coreentity.Base{
			Id:         valobj.Id(row.Id),
			CreateTime: valobj.NewTimeFrom(row.CreatedAt),
			UpdateTime: valobj.NewTimeFrom(row.UpdatedAt),
		},
		UserId: row.UserId,
		Name:   row.Name,
	}
}

func FavouritesFromSchema(rows []*schema.Favourite) []*entity.Favourite {
	out := make([]*entity.Favourite, 0, len(rows))
	for i := range rows {
		out = append(out, FavouriteFromSchema(rows[i]))
	}
	return out
}

func FavouriteNotebookToSchema(row *entity.FavouriteNotebook) *schema.FavouriteNotebook {
	return &schema.FavouriteNotebook{
		FavouriteId:  row.FavouriteId,
		NotebookId:   row.NotebookId,
		FavouritedAt: row.FavouritedAt.Value(),
	}
}

func FavouriteNotebookFromSchema(row *schema.FavouriteNotebook) *entity.FavouriteNotebook {
	return &entity.FavouriteNotebook{
		FavouriteId:  valobj.Id(row.FavouriteId),
		NotebookId:   valobj.Id(row.NotebookId),
		FavouritedAt: valobj.NewTimeFrom(row.FavouritedAt),
	}
}

func FavouriteNotebooksFromSchema(rows []*schema.FavouriteNotebook) []*entity.FavouriteNotebook {
	out := make([]*entity.FavouriteNotebook, 0, len(rows))
	for i := range rows {
		out = append(out, FavouriteNotebookFromSchema(rows[i]))
	}
	return out
}
