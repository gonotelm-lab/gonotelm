package notebook

import (
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	pinapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/pin"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type Deps struct {
	NotebookRepo          notebookrepo.Repository
	MemberRepo            notebookrepo.MemberRepository
	PinRepo               pinrepo.Repository
	NotebookAccessService *notebookservice.AccessService
	SourceRepo            sourcerepo.Repository
	EventBus              eventbus.Publisher
	DistLock              adapter.DistributedLock
	Summarizer            adapter.Summarizer
	AIDescriptionRepo     notebookrepo.AIDescriptionRepository
	Cleaner               *cleanup.Cleaner
	UserRepo              identityrepo.UserRepository
	// ObjectStore 仅注入 app handler；API handler 本身不解析 StoreKey。
	// 封面 presign/confirm/delete 需要完整 ObjectStore 能力
	ObjectStore adapter.ObjectStore
	KeyFactory  adapter.StoreKeyFactory
}

type Server struct {
	getNotebookHandler                 *notebookapp.GetNotebookHandler
	createNotebookHandler              *notebookapp.CreateNotebookHandler
	listNotebooksHandler               *notebookapp.ListNotebooksHandler
	deleteNotebookHandler              *notebookapp.DeleteNotebookHandler
	updateNotebookHandler              *notebookapp.UpdateNotebookHandler
	generateNotebookDescriptionHandler *notebookapp.GenerateDescriptionHandler

	getNotebookAccessHandler        *notebookapp.GetNotebookAccessHandler
	getNotebookSharingHandler       *notebookapp.GetNotebookSharingHandler
	updateNotebookVisibilityHandler *notebookapp.UpdateNotebookVisibilityHandler
	upsertNotebookMemberHandler     *notebookapp.UpsertNotebookMemberHandler
	deleteNotebookMemberHandler     *notebookapp.DeleteNotebookMemberHandler

	pinNotebookHandler   *pinapp.PinNotebookHandler
	unpinNotebookHandler *pinapp.UnpinNotebookHandler

	coverUploadPresignHandler *notebookapp.CoverUploadPresignHandler
	confirmCoverUploadHandler *notebookapp.ConfirmCoverUploadHandler
	deleteCoverHandler        *notebookapp.DeleteCoverHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		getNotebookHandler: notebookapp.NewGetNotebookHandler(
			deps.NotebookRepo, deps.NotebookAccessService, deps.UserRepo, deps.ObjectStore, deps.PinRepo,
		),
		createNotebookHandler: notebookapp.NewCreateNotebookHandler(deps.NotebookRepo, deps.EventBus),
		listNotebooksHandler: notebookapp.NewListNotebooksHandler(
			deps.NotebookRepo, deps.MemberRepo, deps.PinRepo, deps.UserRepo, deps.ObjectStore,
		),
		pinNotebookHandler: pinapp.NewPinNotebookHandler(
			deps.NotebookRepo, deps.MemberRepo, deps.PinRepo,
		),
		unpinNotebookHandler: pinapp.NewUnpinNotebookHandler(deps.PinRepo),
		deleteNotebookHandler: notebookapp.NewDeleteNotebookHandler(
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.MemberRepo,
			deps.UserRepo,
			deps.EventBus,
			deps.Cleaner,
			deps.ObjectStore,
		),
		updateNotebookHandler: notebookapp.NewUpdateNotebookHandler(deps.NotebookRepo, deps.NotebookAccessService, deps.UserRepo),
		generateNotebookDescriptionHandler: notebookapp.NewGenerateDescriptionHandler(
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.SourceRepo,
			deps.UserRepo,
			deps.DistLock,
			deps.Summarizer,
			deps.AIDescriptionRepo,
		),

		getNotebookAccessHandler: notebookapp.NewGetNotebookAccessHandler(deps.NotebookAccessService),
		getNotebookSharingHandler: notebookapp.NewGetNotebookSharingHandler(
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.MemberRepo,
			deps.UserRepo,
			deps.ObjectStore,
		),
		updateNotebookVisibilityHandler: notebookapp.NewUpdateNotebookVisibilityHandler(deps.NotebookRepo, deps.NotebookAccessService, deps.UserRepo),
		upsertNotebookMemberHandler:     notebookapp.NewUpsertNotebookMemberHandler(deps.NotebookRepo, deps.NotebookAccessService, deps.MemberRepo, deps.UserRepo),
		deleteNotebookMemberHandler: notebookapp.NewDeleteNotebookMemberHandler(
			deps.NotebookRepo, deps.NotebookAccessService, deps.MemberRepo, deps.UserRepo, deps.EventBus,
		),

		coverUploadPresignHandler: notebookapp.NewCoverUploadPresignHandler(
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.UserRepo,
			deps.ObjectStore,
			deps.KeyFactory,
			deps.DistLock,
			deps.Cleaner,
		),
		confirmCoverUploadHandler: notebookapp.NewConfirmCoverUploadHandler(
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.UserRepo,
			deps.ObjectStore,
			deps.DistLock,
		),
		deleteCoverHandler: notebookapp.NewDeleteCoverHandler(
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.UserRepo,
			deps.ObjectStore,
			deps.DistLock,
		),
	}
}
