package schema

import (
	"strings"

	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type CreateNotebookRequest struct {
	Name string `json:"name" validate:"max=128"`
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

// NotebookFlagsResponse notebook 位标记的 API 展开。
type NotebookFlagsResponse struct {
	CustomizeDescription bool `json:"customize_description"`
}

// NotebookResponse Get / List 共用的 notebook 视图。
type NotebookResponse struct {
	Id          string                 `json:"id"`
	Name        string                 `json:"name"`
	Desc        string                 `json:"desc"`
	Flags       NotebookFlagsResponse  `json:"flags"`
	SourceCount int64                  `json:"source_count"`
	Visibility  string                 `json:"visibility"`
	UpdatedAt   int64                  `json:"updated_at"` // unix ms
	CreatedAt   int64                  `json:"created_at"` // unix ms
	Owner       *NotebookOwnerResponse `json:"owner,omitempty"`
}

func NewNotebookResponse(notebook *notebookapp.Notebook) *NotebookResponse {
	return &NotebookResponse{
		Id:   notebook.Id.String(),
		Name: notebook.Name,
		Desc: notebook.Description,
		Flags: NotebookFlagsResponse{
			CustomizeDescription: notebook.CustomizeDescription(),
		},
		SourceCount: notebook.SourceCount,
		Visibility:  string(notebook.Visibility),
		Owner:       NewNotebookOwnerResponse(notebook.Owner),
		UpdatedAt:   notebook.UpdateTime.Value(),
		CreatedAt:   notebook.CreateTime.Value(),
	}
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

// 首页列出笔记本时 可指定筛选的scope；owned 与 shared 互斥。
type ListNotebooksScope string

const (
	ListNotebooksScopeOwned  ListNotebooksScope = "owned"
	ListNotebooksScopeShared ListNotebooksScope = "shared"
)

func (s ListNotebooksScope) IsValid() bool {
	return s == ListNotebooksScopeOwned || s == ListNotebooksScopeShared
}

func (s ListNotebooksScope) ToScope() notebookapp.AccessScope {
	if s == ListNotebooksScopeShared {
		return notebookapp.AccessScopeShared
	}

	return notebookapp.AccessScopeOwned
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
		r.Scope = ListNotebooksScopeOwned
	} else if !r.Scope.IsValid() {
		return errors.ErrParams.Msgf("invalid scope: %s", r.Scope)
	}

	return nil
}

type ListNotebooksResponse struct {
	Notebooks []*NotebookResponse `json:"notebooks"`
	Limit     int                 `json:"limit"`
	Offset    int                 `json:"offset"`
	HasMore   bool                `json:"has_more"`
}

type NotebookOwnerResponse struct {
	UserId    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

func NewNotebookOwnerResponse(owner *notebookapp.NotebookOwner) *NotebookOwnerResponse {
	if owner == nil {
		return nil
	}
	return &NotebookOwnerResponse{
		UserId:    owner.Id.String(),
		Nickname:  owner.Nickname,
		AvatarURL: owner.AvatarURL,
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
	Id                   uuid.UUID `path:"id,required"`
	Name                 *string   `json:"name"                   validate:"omitempty,min=0,max=128"`
	CustomizeDescription *bool     `json:"customize_description"`
	Desc                 *string   `json:"desc"                   validate:"omitempty,max=1024"`
}

func (r *UpdateNotebookRequest) Validate() error {
	if r.Name == nil && r.CustomizeDescription == nil && r.Desc == nil {
		return errors.ErrParams.Msg("at least one of name, customize_description, desc is required")
	}
	if r.CustomizeDescription != nil && *r.CustomizeDescription {
		if r.Desc == nil || strings.TrimSpace(*r.Desc) == "" {
			return errors.ErrParams.Msg("desc is required when customize_description is true")
		}
	}
	if r.CustomizeDescription == nil && r.Desc != nil {
		return errors.ErrParams.Msg("desc requires customize_description")
	}
	return nil
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

func (r *UpdateNotebookSharingRequest) Validate() error {
	if vo.Visibility(r.Visibility).IsInvalid() {
		return errors.ErrParams.Msgf("invalid visibility %s", r.Visibility)
	}

	return nil
}

type UpsertNotebookMemberRequest struct {
	Id    uuid.UUID `path:"id,required"`
	Email string    `json:"email" validate:"required,email"`
	Role  string    `json:"role"  validate:"required"`
}

func (r *UpsertNotebookMemberRequest) Validate() error {
	if vo.Role(r.Role).IsInvalid() {
		return errors.ErrParams.Msgf("invalid role %s", r.Role)
	}
	return nil
}

type UpsertNotebookMemberResponse struct {
	UserId string `json:"user_id"`
	Role   string `json:"role"`
}

type DeleteNotebookMemberRequest struct {
	Id     uuid.UUID  `path:"id,required"` // notebook id
	UserId valobj.Uid `path:"user_id,required"`
}
