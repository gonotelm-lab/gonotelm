// Package source 提供来源（notebook 下的文档/链接/笔记）相关接口：
// 创建、上传、轮询状态、重试解析，以及来源文档片段与来源列表。
package source

import (
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

// Deps 来源接口所需依赖，由外层 notelm.ServerDeps 裁剪后传入
type Deps struct {
	SourceRepo    sourcerepo.Repository
	NotebookRepo  notebookrepo.Repository
	ObjectStore   adapter.ObjectStore
	KeyFactory    adapter.StoreKeyFactory
	SourceDocRepo sourcerepo.SourceDocRepository
	EventBus      eventbus.Publisher
}

// Server 来源接口的处理器集合
type Server struct {
	getSourceHandler              *sourceapp.GetSourceHandler
	createSourceHandler           *sourceapp.CreateSourceHandler
	deleteSourceHandler           *sourceapp.DeleteSourceHandler
	presignUploadFileHandler      *sourceapp.PresignUploadFileHandler
	pollSourceStatusHandler       *sourceapp.PollSourceStatusHandler
	retrySourcePreparationHandler *sourceapp.RetrySourcePreparationHandler
	updateSourceTitleHandler      *sourceapp.UpdateSourceTitleHandler

	getSourceDocHandler      *sourceapp.GetSourceDocHandler
	batchGetSourceDocHandler *sourceapp.BatchGetSourceDocsHandler

	listSourcesHandler *sourceapp.ListSourcesHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		getSourceHandler:              sourceapp.NewGetSourceHandler(deps.SourceRepo, deps.ObjectStore),
		createSourceHandler:           sourceapp.NewCreateSourceHandler(deps.SourceRepo, deps.NotebookRepo, deps.EventBus),
		deleteSourceHandler:           sourceapp.NewDeleteSourceHandler(deps.SourceRepo, deps.EventBus),
		presignUploadFileHandler:      sourceapp.NewPresignUploadFileHandler(deps.SourceRepo, deps.ObjectStore, deps.KeyFactory),
		pollSourceStatusHandler:       sourceapp.NewPollSourceStatusHandler(deps.SourceRepo, deps.ObjectStore, deps.EventBus),
		retrySourcePreparationHandler: sourceapp.NewRetrySourcePreparationHandler(deps.SourceRepo, deps.EventBus),
		updateSourceTitleHandler:      sourceapp.NewUpdateSourceTitleHandler(deps.SourceRepo),

		getSourceDocHandler:      sourceapp.NewGetSourceDocHandler(deps.SourceRepo, deps.SourceDocRepo),
		batchGetSourceDocHandler: sourceapp.NewBatchGetSourceDocsHandler(deps.SourceRepo, deps.SourceDocRepo),

		listSourcesHandler: sourceapp.NewListSourcesHandler(deps.NotebookRepo, deps.SourceRepo, deps.ObjectStore),
	}
}
