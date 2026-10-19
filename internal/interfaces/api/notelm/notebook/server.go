package notebook

import (
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type Deps struct {
	NotebookRepo          notebookrepo.Repository
	MemberRepo            notebookrepo.MemberRepository
	NotebookAccessService *notebookservice.AccessService
	SourceRepo            sourcerepo.Repository
	EventBus              eventbus.Publisher
	DistLock              adapter.DistributedLock
	Summarizer            adapter.Summarizer
	Cleaner               *cleanup.Cleaner
	UserRepo              identityrepo.UserRepository
	ObjectStore           adapter.ObjectPublicURLer
}

type Server struct {
	getNotebookHandler                 *notebookapp.GetNotebookHandler
	createNotebookHandler              *notebookapp.CreateNotebookHandler
	listNotebooksHandler               *notebookapp.ListNotebooksHandler
	deleteNotebookHandler              *notebookapp.DeleteNotebookHandler
	updateNotebookNameHandler          *notebookapp.UpdateNotebookNameHandler
	generateNotebookDescriptionHandler *notebookapp.GenerateDescriptionHandler

	getNotebookAccessHandler        *notebookapp.GetNotebookAccessHandler
	getNotebookSharingHandler       *notebookapp.GetNotebookSharingHandler
	updateNotebookVisibilityHandler *notebookapp.UpdateNotebookVisibilityHandler
	upsertNotebookMemberHandler     *notebookapp.UpsertNotebookMemberHandler
	deleteNotebookMemberHandler     *notebookapp.DeleteNotebookMemberHandler

	userRepo    identityrepo.UserRepository
	objectStore adapter.ObjectPublicURLer
}

func NewServer(deps Deps) *Server {
	return &Server{
		getNotebookHandler:        notebookapp.NewGetNotebookHandler(deps.NotebookRepo, deps.NotebookAccessService),
		createNotebookHandler:     notebookapp.NewCreateNotebookHandler(deps.NotebookRepo, deps.EventBus),
		listNotebooksHandler:      notebookapp.NewListNotebooksHandler(deps.NotebookRepo, deps.MemberRepo),
		deleteNotebookHandler:     notebookapp.NewDeleteNotebookHandler(deps.NotebookRepo, deps.NotebookAccessService, deps.MemberRepo, deps.EventBus, deps.Cleaner),
		updateNotebookNameHandler: notebookapp.NewUpdateNotebookNameHandler(deps.NotebookRepo, deps.NotebookAccessService),
		generateNotebookDescriptionHandler: notebookapp.NewGenerateDescriptionHandler(
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.SourceRepo,
			deps.DistLock,
			deps.Summarizer,
		),

		getNotebookAccessHandler:        notebookapp.NewGetNotebookAccessHandler(deps.NotebookAccessService),
		getNotebookSharingHandler:       notebookapp.NewGetNotebookSharingHandler(deps.NotebookRepo, deps.NotebookAccessService, deps.MemberRepo, deps.UserRepo, deps.ObjectStore),
		updateNotebookVisibilityHandler: notebookapp.NewUpdateNotebookVisibilityHandler(deps.NotebookRepo, deps.NotebookAccessService),
		upsertNotebookMemberHandler:     notebookapp.NewUpsertNotebookMemberHandler(deps.NotebookRepo, deps.NotebookAccessService, deps.MemberRepo, deps.UserRepo),
		deleteNotebookMemberHandler:     notebookapp.NewDeleteNotebookMemberHandler(deps.NotebookRepo, deps.NotebookAccessService, deps.MemberRepo),

		userRepo:    deps.UserRepo,
		objectStore: deps.ObjectStore,
	}
}
