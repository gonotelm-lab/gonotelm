package schema

import (
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type CreateNotebookRequest struct {
	Name string `json:"name" validate:"max=128"`
	Desc string `json:"desc" validate:"max=1024"`
}

type CreateNotebookResponse struct {
	Id string `json:"id"`
}

type GetNotebookRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (r *GetNotebookRequest) Validate() error {
	return nil
}

type GetNotebookResponse struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Desc        string `json:"desc"`
	SourceCount int64  `json:"source_count"`
	UpdatedAt   int64  `json:"updated_at"` // unix ms
	CreatedAt   int64  `json:"created_at"` // unix ms
}

type ListNotebooksSortBy string

const (
	ListNotebooksSortByLastActive ListNotebooksSortBy = "last_active"
	ListNotebooksSortByCreateTime ListNotebooksSortBy = "create_time"
)

func (s ListNotebooksSortBy) ToSortBy() notebookapp.SortBy {
	switch s {
	case ListNotebooksSortByLastActive:
		return notebookapp.SortByLastActive
	case ListNotebooksSortByCreateTime:
		return notebookapp.SortByCreateTime
	}

	return notebookapp.SortByCreateTime
}

type ListNotebooksRequest struct {
	Limit  int                 `query:"limit"   validate:"omitempty,min=1,max=100"`
	Offset int                 `query:"offset"  validate:"min=0"`
	SortBy ListNotebooksSortBy `query:"sort_by" validate:"omitempty,oneof=last_active create_time"`
}

const defaultNotebooksListLimit = 20

func (r *ListNotebooksRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultNotebooksListLimit
	}

	if r.SortBy == "" {
		r.SortBy = ListNotebooksSortByCreateTime
	}

	return nil
}

type ListNotebooksResponse struct {
	Notebooks []*ListNotebookItemResponse `json:"notebooks"`
	Limit     int                         `json:"limit"`
	Offset    int                         `json:"offset"`
	HasMore   bool                        `json:"has_more"`
}

type ListNotebookItemResponse struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Desc        string `json:"desc"`
	SourceCount int64  `json:"source_count"`
	UpdatedAt   int64  `json:"updated_at"` // unix ms
	CreatedAt   int64  `json:"created_at"` // unix ms
}

type ListNotebookSourcesRequest struct {
	Id     uuid.UUID `path:"id,required"`
	Limit  int       `query:"limit"      validate:"omitempty,min=1,max=50"`
	Offset int       `query:"offset"     validate:"min=0"`
}

const defaultNotebookSourcesLimit = 50

func (r *ListNotebookSourcesRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultNotebookSourcesLimit
	}
	return nil
}

type ListNotebookSourcesResponse struct {
	Sources []*Source `json:"sources"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset"`
	HasMore bool      `json:"has_more"`
}

type UpdateNotebookRequest struct {
	Id   uuid.UUID `path:"id,required"`
	Name string    `json:"name"        validate:"min=0,max=128"`
}

type GetNotebookChatRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (r *GetNotebookChatRequest) Validate() error {
	return nil
}

type GetNotebookChatResponse struct {
	ChatId string `json:"chat_id"`
}

type DeleteNotebookRequest struct {
	Id uuid.UUID `path:"id,required"`
}
