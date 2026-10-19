package notebook

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgctx "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
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

	http.OkResp(c, schema.GetNotebookResponse{
		Id:          notebook.Id.String(),
		Name:        notebook.Name,
		Desc:        notebook.Description,
		SourceCount: notebook.SourceCount,
		Visibility:  string(notebook.Visibility),
		UpdatedAt:   notebook.UpdateTime.Value(),
		CreatedAt:   notebook.CreateTime.Value(),
	})
}

// ListNotebooks 返回当前用户可见的 notebook 列表：
//   - scope=all（默认）：owner 的 + 被显式授权的（viewer/collaborator）；
//   - scope=owned / shared：只返回对应部分。
//
// 仅通过 public 获得访问的 notebook 不进入列表；
// 需要当前用户的角色时调用 GET /notebooks/:id/access。
// 所有列表项都会附带 owner 概要。
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

	// 所有列表项统一返回 owner 概要
	ownerIds := make([]valobj.Uid, 0, len(result.Notebooks))
	for _, notebook := range result.Notebooks {
		ownerIds = append(ownerIds, notebook.OwnerId)
	}
	ownerIds = pkgslices.Unique(ownerIds)

	ownerMap := make(map[valobj.Uid]*schema.NotebookOwnerResponse, len(ownerIds))
	// 去重后一次性批量取 owner 用户信息，避免逐条查询。
	if len(ownerIds) > 0 {
		users, err := s.userRepo.BatchGetById(ctx, ownerIds)
		if err != nil {
			http.ErrResp(c, errors.WithMessage(err, "get notebook owners failed"))
			return
		}
		for _, user := range users {
			ownerMap[user.Id] = &schema.NotebookOwnerResponse{
				UserId:    user.Id.String(),
				Nickname:  user.Nickname,
				AvatarURL: s.avatarURL(ctx, user.Avatar),
			}
		}
	}

	notebooks := make([]*schema.ListNotebookItemResponse, 0, len(result.Notebooks))
	for _, notebook := range result.Notebooks {
		notebooks = append(notebooks, schema.NewListNotebookItemResponse(notebook, ownerMap[notebook.OwnerId]))
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
