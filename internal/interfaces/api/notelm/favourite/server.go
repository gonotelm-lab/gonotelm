package favourite

import (
	favouriteapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/favourite"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	favouriterepo "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	favouriteservice "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/service"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
)

type Deps struct {
	FavouriteRepo         favouriterepo.Repository
	FavouriteNotebookRepo favouriterepo.NotebookRepository
	FavouriteService      *favouriteservice.Service
	NotebookRepo          notebookrepo.Repository
	NotebookAccessService *notebookservice.AccessService
	UserRepo              identrepo.UserRepository
	ObjectStore           adapter.ObjectPublicURLer
	DistLock              adapter.DistributedLock
}

type Server struct {
	createHandler         *favouriteapp.CreateFavouriteHandler
	renameHandler         *favouriteapp.RenameFavouriteHandler
	deleteHandler         *favouriteapp.DeleteFavouriteHandler
	listHandler           *favouriteapp.ListFavouritesHandler
	addNotebookHandler    *favouriteapp.AddNotebookHandler
	removeNotebookHandler *favouriteapp.RemoveNotebooksHandler
	listNotebookHandler   *favouriteapp.ListNotebooksHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		createHandler: favouriteapp.NewCreateFavouriteHandler(
			deps.FavouriteRepo, deps.FavouriteService, deps.DistLock,
		),
		renameHandler: favouriteapp.NewRenameFavouriteHandler(
			deps.FavouriteRepo, deps.FavouriteNotebookRepo,
		),
		deleteHandler: favouriteapp.NewDeleteFavouriteHandler(deps.FavouriteRepo),
		listHandler: favouriteapp.NewListFavouritesHandler(
			deps.FavouriteRepo, deps.FavouriteNotebookRepo,
		),
		addNotebookHandler: favouriteapp.NewAddNotebookHandler(
			deps.FavouriteRepo,
			deps.FavouriteNotebookRepo,
			deps.FavouriteService,
			deps.NotebookAccessService,
			deps.DistLock,
		),
		removeNotebookHandler: favouriteapp.NewRemoveNotebooksHandler(
			deps.FavouriteRepo, deps.FavouriteNotebookRepo,
		),
		listNotebookHandler: favouriteapp.NewListNotebooksHandler(
			deps.FavouriteRepo,
			deps.FavouriteNotebookRepo,
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.UserRepo,
			deps.ObjectStore,
		),
	}
}
