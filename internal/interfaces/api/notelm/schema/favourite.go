package schema

import (
	favouriteapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/favourite"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const defaultFavouritesListLimit = 20

type CreateFavouriteRequest struct {
	Name string `json:"name"`
}

func (r *CreateFavouriteRequest) Validate() error {
	return nil
}

type CreateFavouriteResponse struct {
	Id string `json:"id"`
}

type FavouriteResponse struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	NotebookCount int64  `json:"notebook_count"`
	CreatedAt     int64  `json:"created_at"`
}

func NewFavouriteResponse(v *favouriteapp.FavouriteView) *FavouriteResponse {
	return &FavouriteResponse{
		Id:            v.Id.String(),
		Name:          v.Name,
		NotebookCount: v.NotebookCount,
		CreatedAt:     v.CreatedAt,
	}
}

type ListFavouritesRequest struct {
	Limit  int `query:"limit"  validate:"omitempty,min=1,max=100"`
	Offset int `query:"offset" validate:"min=0"`
}

func (r *ListFavouritesRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultFavouritesListLimit
	}
	return nil
}

type ListFavouritesResponse struct {
	Favourites []*FavouriteResponse `json:"favourites"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
	HasMore    bool                 `json:"has_more"`
}

type RenameFavouriteRequest struct {
	Id   uuid.UUID `path:"id,required"`
	Name string    `json:"name"`
}

func (r *RenameFavouriteRequest) Validate() error {
	return nil
}

type DeleteFavouriteRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (r *DeleteFavouriteRequest) Validate() error {
	return nil
}

type ListFavouriteNotebooksRequest struct {
	Id     uuid.UUID `path:"id,required"`
	Limit  int       `query:"limit"  validate:"omitempty,min=1,max=100"`
	Offset int       `query:"offset" validate:"min=0"`
}

func (r *ListFavouriteNotebooksRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultFavouritesListLimit
	}
	return nil
}

type FavouriteNotebookItemResponse struct {
	NotebookId   string            `json:"notebook_id"`
	Invalid      bool              `json:"invalid"`
	FavouritedAt int64             `json:"favourited_at"`
	Notebook     *NotebookResponse `json:"notebook,omitempty"`
}

func NewFavouriteNotebookItemResponse(item *favouriteapp.FavouriteNotebookItem) *FavouriteNotebookItemResponse {
	resp := &FavouriteNotebookItemResponse{
		NotebookId:   item.NotebookId.String(),
		Invalid:      item.Invalid,
		FavouritedAt: item.FavouritedAt,
	}
	if !item.Invalid && item.Notebook != nil {
		resp.Notebook = NewNotebookResponse(item.Notebook)
	}
	return resp
}

type ListFavouriteNotebooksResponse struct {
	Items   []*FavouriteNotebookItemResponse `json:"items"`
	Limit   int                              `json:"limit"`
	Offset  int                              `json:"offset"`
	HasMore bool                             `json:"has_more"`
}

type AddFavouriteNotebookRequest struct {
	Id         uuid.UUID `path:"id,required"`
	NotebookId uuid.UUID `json:"notebook_id" validate:"required"`
}

func (r *AddFavouriteNotebookRequest) Validate() error {
	if r.NotebookId.IsZero() {
		return errors.ErrParams.Msg("notebook_id is required")
	}
	return nil
}

type RemoveFavouriteNotebooksRequest struct {
	Id          uuid.UUID   `path:"id,required"`
	NotebookIds []uuid.UUID `json:"notebook_ids"`
}

func (r *RemoveFavouriteNotebooksRequest) Validate() error {
	if len(r.NotebookIds) == 0 {
		return errors.ErrParams.Msg("notebook_ids is required")
	}
	return nil
}
