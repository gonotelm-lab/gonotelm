package notebook

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookvo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

// 获取某个用户对notebook的操作权限
func (s *Server) GetNotebookAccess(ctx context.Context, c *app.RequestContext) {
	var req schema.GetNotebookRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	access, err := s.getNotebookAccessHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GetNotebookAccessResponse{
		Role:              access.Role.String(),
		CanView:           access.CanView(),
		CanManageContent:  access.CanManageContent(),
		CanManageNotebook: access.CanManageNotebook(),
		CanManageSharing:  access.CanManageSharing(),
	})
}

// 获取notebook的分享情况
func (s *Server) GetNotebookSharing(ctx context.Context, c *app.RequestContext) {
	var req schema.GetNotebookSharingRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.getNotebookSharingHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	members := make([]*schema.NotebookSharingMember, 0, len(result.Members))
	for _, member := range result.Members {
		members = append(members, &schema.NotebookSharingMember{
			UserId:    member.UserId.String(),
			Email:     member.Email,
			Nickname:  member.Nickname,
			AvatarURL: member.AvatarURL,
			Role:      member.Role.String(),
		})
	}

	http.OkResp(c, schema.GetNotebookSharingResponse{
		Visibility: result.Visibility.String(),
		Members:    members,
	})
}

// 修改notebook的visibility
func (s *Server) UpdateNotebookVisibility(ctx context.Context, c *app.RequestContext) {
	var req schema.UpdateNotebookSharingRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	err := s.updateNotebookVisibilityHandler.Handle(ctx, req.Id, notebookvo.Visibility(req.Visibility))
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

// 添加某个用户访问notebook的权限
func (s *Server) UpsertNotebookMember(ctx context.Context, c *app.RequestContext) {
	var req schema.UpsertNotebookMemberRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	member, err := s.upsertNotebookMemberHandler.Handle(ctx, req.Id,
		&notebookapp.UpsertNotebookMemberCommand{
			Email: req.Email,
			Role:  req.Role,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.UpsertNotebookMemberResponse{
		UserId: member.UserId.String(),
		Role:   string(member.Role),
	})
}

// 移除某个用户对notebook的访问权限
func (s *Server) DeleteNotebookMember(ctx context.Context, c *app.RequestContext) {
	var req schema.DeleteNotebookMemberRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.deleteNotebookMemberHandler.Handle(ctx, req.Id, req.UserId); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

func (s *Server) avatarURL(ctx context.Context, avatar valobj.StoreKey) string {
	if !avatar.Valid() {
		return ""
	}

	return s.storeKeyURLExchanger.Exchange(ctx, avatar)
}
