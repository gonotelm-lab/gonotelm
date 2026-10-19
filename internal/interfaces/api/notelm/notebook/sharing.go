package notebook

import (
	"context"
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookvo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

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
		Role:              string(access.Role),
		CanView:           access.CanView(),
		CanManageContent:  access.CanManageContent(),
		CanManageNotebook: access.CanManageNotebook(),
		CanManageSharing:  access.CanManageSharing(),
	})
}

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
			Role:      string(member.Role),
		})
	}

	http.OkResp(c, schema.GetNotebookSharingResponse{
		Visibility: string(result.Visibility),
		Members:    members,
	})
}

func (s *Server) UpdateNotebookSharing(ctx context.Context, c *app.RequestContext) {
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

func (s *Server) UpsertNotebookMember(ctx context.Context, c *app.RequestContext) {
	var req schema.UpsertNotebookMemberRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	member, err := s.upsertNotebookMemberHandler.Handle(ctx, req.Id, &notebookapp.UpsertNotebookMemberCommand{
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

func (s *Server) DeleteNotebookMember(ctx context.Context, c *app.RequestContext) {
	var req schema.DeleteNotebookMemberRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	userId, err := valobj.NewUidFromString(req.UserId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid user id: %s", req.UserId))
		return
	}

	if err := s.deleteNotebookMemberHandler.Handle(ctx, req.Id, userId); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

func (s *Server) avatarURL(ctx context.Context, avatar valobj.StoreKey) string {
	if !avatar.Valid() {
		return ""
	}

	url, err := s.objectStore.PublicURL(ctx, avatar)
	if err != nil {
		slog.WarnContext(ctx, "resolve avatar public url failed",
			slog.String("avatar", avatar.String()),
			slog.Any("err", err),
		)
		return ""
	}

	return url
}
