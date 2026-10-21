package notebook

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgctx "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

// Create new notebook
func (s *Server) CreateNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.CreateNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	userId := pkgctx.GetUserId(ctx)
	id, err := s.createNotebookHandler.Handle(ctx,
		&notebookapp.CreateNotebookHandleCommand{
			Name:    req.Name,
			Desc:    req.Desc,
			OwnerId: userId,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.CreateNotebookResponse{Id: id.String()})
}

func (s *Server) GetNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.GetNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	notebook, err := s.getNotebookHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}
	result := schema.GetNotebookResponse{
		Id:          notebook.Id.String(),
		Name:        notebook.Name,
		Desc:        notebook.Description,
		SourceCount: notebook.SourceCount,
		Visibility:  string(notebook.Visibility),
		UpdatedAt:   notebook.UpdateTime.Value(),
		CreatedAt:   notebook.CreateTime.Value(),
	}

	result.Owner = schema.NewNotebookOwnerResponse(notebook.Owner)
	result.Owner.AvatarURL = s.avatarURL(ctx, notebook.Owner.Avatar)

	http.OkResp(c, result)
}

// ListNotebooks 按 scope 返回当前用户的 notebook 列表，owned 与 shared 互斥：
//   - scope=owned（默认）：当前用户作为 owner 的 notebook；
//   - scope=shared：显式授权给当前用户的 notebook（不包含本人拥有的）。
//
// 仅通过 public 获得访问的 notebook 不进入列表；
// 需要当前用户的角色时调用 GET /notebooks/:id/access。
// 列表在数据库侧分页，所有列表项都会附带 owner 概要。
func (s *Server) ListNotebooks(ctx context.Context, c *app.RequestContext) {
	var req schema.ListNotebooksRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	userId := pkgctx.GetUserId(ctx)
	result, err := s.listNotebooksHandler.Handle(ctx,
		&notebookapp.ListNotebooksHandleQuery{
			UserId: userId,
			Scope:  req.Scope.ToScope(),
			Limit:  req.Limit,
			Offset: req.Offset,
			SortBy: req.SortBy.ToSortBy(),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	// // 所有列表项统一返回 owner 概要
	notebooks := make([]*schema.ListNotebookItemResponse, 0, len(result.Notebooks))
	for _, notebook := range result.Notebooks {
		n := schema.NewListNotebookItemResponse(notebook)
		n.Owner.AvatarURL = s.avatarURL(ctx, notebook.Owner.Avatar)
		notebooks = append(notebooks, n)
	}

	http.OkResp(c, schema.ListNotebooksResponse{
		Notebooks: notebooks,
		Limit:     req.Limit,
		Offset:    req.Offset,
		HasMore:   result.HasMore,
	})
}

func (s *Server) UpdateNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.UpdateNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.updateNotebookNameHandler.Handle(ctx, req.Id, req.Name)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func (s *Server) DeleteNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.DeleteNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.deleteNotebookHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

func (s *Server) GenerateNotebookDescription(ctx context.Context, c *app.RequestContext) {
	var req schema.GenerateNotebookDescriptionRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	desc, err := s.generateNotebookDescriptionHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GenerateNotebookDescriptionResponse{
		Desc: desc,
	})
}
