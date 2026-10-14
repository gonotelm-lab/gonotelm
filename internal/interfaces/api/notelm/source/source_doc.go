package source

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

// 获取来源的文档片段
func (s *Server) GetSourceDoc(ctx context.Context, c *app.RequestContext) {
	var req schema.GetSourceDocRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.getSourceDocHandler.Handle(ctx,
		&sourceapp.GetSourceDocHandleQuery{
			SourceId: req.Id,
			DocId:    req.DocId,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.ToGetSourceDocResponse(result.SourceId, result.SourceTitle, result.Doc))
}

// 批量获取来源文档片段
func (s *Server) BatchGetSourceDocs(ctx context.Context, c *app.RequestContext) {
	var req schema.BatchGetSourceDocsRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.batchGetSourceDocHandler.Handle(ctx,
		&sourceapp.BatchGetSourceDocsHandleQuery{
			SourceId: req.Id,
			DocIds:   req.DocIds(),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	docs := make([]*schema.GetSourceDocResponse, 0, len(result.Docs))
	for _, doc := range result.Docs {
		docs = append(docs, schema.ToGetSourceDocResponse(result.SourceId, result.SourceTitle, doc))
	}

	http.OkResp(c, &schema.BatchGetSourceDocsResponse{
		Docs: docs,
	})
}
