package notelm

import (
	"context"
	"io"

	userapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/user"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) registerUserRoutes(g *route.RouterGroup) {
	userGroup := g.Group("/user")
	{
		// GET /api/v1/user/me
		userGroup.GET("/me", s.GetMe)
		// PATCH /api/v1/user/me
		userGroup.PATCH("/me", s.UpdateMe)
		// POST /api/v1/user/me/avatar
		userGroup.POST("/me/avatar", s.UpdateAvatar)
	}
}

func (s *Server) GetMe(ctx context.Context, c *app.RequestContext) {
	resp, err := s.getMeHandler.Handle(ctx)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.MeResponse{
		UserId:      resp.UserId,
		Nickname:    resp.Nickname,
		AvatarUrl:   resp.AvatarUrl,
		CreatedAt:   resp.CreatedAt.Value(),
		UpdatedAt:   resp.UpdatedAt.Value(),
		LoginSource: resp.LoginSource.String(),
	})
}

func (s *Server) UpdateMe(ctx context.Context, c *app.RequestContext) {
	var req schema.UpdateMeRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	_, err = s.updateProfileHandler.Handle(ctx, &userapp.UpdateProfileCommand{
		Nickname: req.Nickname,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

func (s *Server) UpdateAvatar(ctx context.Context, c *app.RequestContext) {
	if length := c.Request.Header.ContentLength(); length > 0 && int64(length) > schema.MaxAvatarBodyBytes {
		http.ErrResp(c, errors.ErrParams.Msg("avatar too large"))
		return
	}

	fileHeader, err := c.Request.FormFile(schema.AvatarFormField)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msg("avatar file is required"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msg("open avatar file failed"))
		return
	}
	defer file.Close() //nolint:errcheck

	content, err := io.ReadAll(io.LimitReader(file, schema.MaxAvatarBytes+1))
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msg("read avatar file failed"))
		return
	}

	url, err := s.updateAvatarHandler.Handle(ctx, content)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.UpdateAvatarResponse{AvatarUrl: url})
}
