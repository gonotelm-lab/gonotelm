package notebook

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

// 下发上传封面的上传链接
func (s *Server) CoverUploadPresign(ctx context.Context, c *app.RequestContext) {
	var req schema.NotebookCoverUploadRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.coverUploadPresignHandler.Handle(ctx,
		&notebookapp.CoverUploadPresignHandleCommand{
			NotebookId:  req.Id,
			Size:        req.Size,
			ContentType: req.ContentType,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, &schema.NotebookCoverUploadResponse{
		StoreKey: result.StoreKey,
		Url:      result.Presign.Url,
		Method:   result.Presign.Method,
		Forms:    result.Presign.Forms,
		Headers:  result.Presign.Headers,
	})
}

// 把上传完成的在途 key 转为生效封面 封面确认
func (s *Server) ConfirmCoverUpload(ctx context.Context, c *app.RequestContext) {
	var req schema.ConfirmNotebookCoverRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.confirmCoverUploadHandler.Handle(ctx,
		&notebookapp.ConfirmCoverUploadCommand{
			NotebookId: req.Id,
			StoreKey:   req.StoreKey,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, &schema.ConfirmNotebookCoverResponse{
		CoverUrl: result.CoverUrl,
	})
}

// 移除封面
func (s *Server) DeleteNotebookCover(ctx context.Context, c *app.RequestContext) {
	var req schema.DeleteNotebookCoverRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.deleteCoverHandler.Handle(ctx, &notebookapp.DeleteCoverCommand{NotebookId: req.Id}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}
