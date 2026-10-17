package notebook

import (
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

// Deps 笔记本接口所需依赖，由外层 notelm.ServerDeps 裁剪后传入
type Deps struct {
	NotebookRepo     notebookrepo.Repository
	SourceRepo       sourcerepo.Repository
	EventBus         eventbus.Publisher
	DistLock         adapter.DistributedLock
	Summarizer       adapter.Summarizer
	CleanupRegistrar *cleanup.NotebookCleanupRegistrar
}

// Server 笔记本接口的处理器集合
type Server struct {
	getNotebookHandler                 *notebookapp.GetNotebookHandler
	createNotebookHandler              *notebookapp.CreateNotebookHandler
	listNotebooksHandler               *notebookapp.ListNotebooksHandler
	deleteNotebookHandler              *notebookapp.DeleteNotebookHandler
	updateNotebookNameHandler          *notebookapp.UpdateNotebookNameHandler
	generateNotebookDescriptionHandler *notebookapp.GenerateDescriptionHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		getNotebookHandler:        notebookapp.NewGetNotebookHandler(deps.NotebookRepo),
		createNotebookHandler:     notebookapp.NewCreateNotebookHandler(deps.NotebookRepo, deps.EventBus),
		listNotebooksHandler:      notebookapp.NewListNotebooksHandler(deps.NotebookRepo),
		deleteNotebookHandler:     notebookapp.NewDeleteNotebookHandler(deps.NotebookRepo, deps.EventBus, deps.CleanupRegistrar),
		updateNotebookNameHandler: notebookapp.NewUpdateNotebookNameHandler(deps.NotebookRepo),
		generateNotebookDescriptionHandler: notebookapp.NewGenerateDescriptionHandler(
			deps.NotebookRepo,
			deps.SourceRepo,
			deps.DistLock,
			deps.Summarizer,
		),
	}
}
