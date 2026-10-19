package schema

import (
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
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
	Visibility  string `json:"visibility"`
	UpdatedAt   int64  `json:"updated_at"` // unix ms
	CreatedAt   int64  `json:"created_at"` // unix ms
}

type GetNotebookAccessResponse struct {
	Role              string `json:"role"`
	CanView           bool   `json:"can_view"`
	CanManageContent  bool   `json:"can_manage_content"`
	CanManageNotebook bool   `json:"can_manage_notebook"`
	CanManageSharing  bool   `json:"can_manage_sharing"`
}

type ListNotebooksSortBy string

const (
	ListNotebooksSortByLastActive ListNotebooksSortBy = "last_active"
	ListNotebooksSortByCreateTime ListNotebooksSortBy = "create_time"
)

func (s ListNotebooksSortBy) IsValid() bool {
	return s == ListNotebooksSortByLastActive || s == ListNotebooksSortByCreateTime
}

func (s ListNotebooksSortBy) ToSortBy() notebookapp.SortBy {
	switch s {
	case ListNotebooksSortByLastActive:
		return notebookapp.SortByLastActive
	case ListNotebooksSortByCreateTime:
		return notebookapp.SortByCreateTime
	}

	return notebookapp.SortByCreateTime
}

type ListNotebooksScope string

const (
	ListNotebooksScopeAll    ListNotebooksScope = "all"
	ListNotebooksScopeOwned  ListNotebooksScope = "owned"
	ListNotebooksScopeShared ListNotebooksScope = "shared"
)

func (s ListNotebooksScope) IsValid() bool {
	return s == ListNotebooksScopeAll || s == ListNotebooksScopeOwned || s == ListNotebooksScopeShared
}

func (s ListNotebooksScope) ToScope() notebookapp.AccessScope {
	switch s {
	case ListNotebooksScopeOwned:
		return notebookapp.AccessScopeOwned
	case ListNotebooksScopeShared:
		return notebookapp.AccessScopeShared
	}

	return notebookapp.AccessScopeAll
}

type ListNotebooksRequest struct {
	Limit  int                 `query:"limit"   validate:"omitempty,min=1,max=100"`
	Offset int                 `query:"offset"  validate:"min=0"`
	SortBy ListNotebooksSortBy `query:"sort_by"`
	Scope  ListNotebooksScope  `query:"scope"`
}

const defaultNotebooksListLimit = 20

func (r *ListNotebooksRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultNotebooksListLimit
	}

	if r.SortBy == "" {
		r.SortBy = ListNotebooksSortByCreateTime
	} else if !r.SortBy.IsValid() {
		return errors.ErrParams.Msgf("invalid sort_by: %s", r.SortBy)
	}

	if r.Scope == "" {
		r.Scope = ListNotebooksScopeAll
	} else if !r.Scope.IsValid() {
		return errors.ErrParams.Msgf("invalid scope: %s", r.Scope)
	}

	return nil
}

type ListNotebooksResponse struct {
	Notebooks []*ListNotebookItemResponse `json:"notebooks"`
	Limit     int                         `json:"limit"`
	Offset    int                         `json:"offset"`
	HasMore   bool                        `json:"has_more"`
}

type NotebookOwnerResponse struct {
	UserId    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type ListNotebookItemResponse struct {
	Id          string                 `json:"id"`
	Name        string                 `json:"name"`
	Desc        string                 `json:"desc"`
	SourceCount int64                  `json:"source_count"`
	Visibility  string                 `json:"visibility"`
	Owner       *NotebookOwnerResponse `json:"owner,omitempty"`
	UpdatedAt   int64                  `json:"updated_at"` // unix ms
	CreatedAt   int64                  `json:"created_at"` // unix ms
}

func NewListNotebookItemResponse(
	notebook *notebookentity.Notebook,
	owner *NotebookOwnerResponse,
) *ListNotebookItemResponse {
	return &ListNotebookItemResponse{
		Id:          notebook.Id.String(),
		Name:        notebook.Name,
		Desc:        notebook.Description,
		SourceCount: notebook.SourceCount,
		Visibility:  string(notebook.Visibility),
		Owner:       owner,
		UpdatedAt:   notebook.UpdateTime.Value(),
		CreatedAt:   notebook.CreateTime.Value(),
	}
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

type GenerateNotebookDescriptionRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (r *GenerateNotebookDescriptionRequest) Validate() error {
	return nil
}

type GenerateNotebookDescriptionResponse struct {
	Desc string `json:"desc"`
}

type GetNotebookSharingRequest struct {
	Id uuid.UUID `path:"id,required"`
}

type NotebookSharingMember struct {
	UserId    string `json:"user_id"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
}

type GetNotebookSharingResponse struct {
	Visibility string                   `json:"visibility"`
	Members    []*NotebookSharingMember `json:"members"`
}

type UpdateNotebookSharingRequest struct {
	Id         uuid.UUID `path:"id,required"`
	Visibility string    `json:"visibility" validate:"required"`
}

type UpsertNotebookMemberRequest struct {
	Id    uuid.UUID `path:"id,required"`
	Email string    `json:"email" validate:"required,email"`
	Role  string    `json:"role"  validate:"required"`
}

type UpsertNotebookMemberResponse struct {
	UserId string `json:"user_id"`
	Role   string `json:"role"`
}

type DeleteNotebookMemberRequest struct {
	Id     uuid.UUID `path:"id,required"`
	UserId string    `path:"user_id,required"`
}
