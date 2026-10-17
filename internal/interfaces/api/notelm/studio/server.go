package studio

import (
	"sync"

	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
)

// Deps 产物接口所需依赖，由外层 notelm.ServerDeps 裁剪后传入
type Deps struct {
	WaitGroup        *sync.WaitGroup
	ArtifactRepo     artifactrepo.Repository
	StylePreviewRepo artifactrepo.StylePreviewRepository
	NotebookRepo     notebookrepo.Repository
	SourceRepo       sourcerepo.Repository
	ChatRepo         chatrepo.ChatRepository
	ChatMessageRepo  chatrepo.MessageRepository
	FlowClient       flow.TaskClient
	Poller           artifactapp.Poller
	EventBus         eventbus.Publisher
	TitleMaker       adapter.TitleMaker
	ObjectStore      adapter.ObjectStore
	KeyFactory       adapter.StoreKeyFactory
	Cleaner          *cleanup.Cleaner
}

// Server 产物接口的处理器集合
type Server struct {
	generateArtifactHandler      *artifactapp.GenerateArtifactHandler
	getArtifactStatusHandler     *artifactapp.GetArtifactStatusHandler
	listNotebookArtifactsHandler *artifactapp.ListArtifactsHandler
	cancelArtifactHandler        *artifactapp.CancelArtifactHandler
	deleteArtifactHandler        *artifactapp.DeleteArtifactHandler
	retryArtifactHandler         *artifactapp.RetryArtifactHandler
	updateArtifactHandler        *artifactapp.UpdateArtifactHandler
	convertNoteToSourceHandler   *artifactapp.ConvertNoteToSourceHandler
	listStylePreviewsHandler     *artifactapp.ListStylePreviewsHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		generateArtifactHandler: artifactapp.NewGenerateArtifactHandler(
			deps.WaitGroup,
			deps.ArtifactRepo,
			deps.NotebookRepo,
			deps.SourceRepo,
			deps.ChatRepo,
			deps.ChatMessageRepo,
			deps.FlowClient,
			deps.Poller,
			deps.EventBus,
			deps.TitleMaker,
		),
		getArtifactStatusHandler:     artifactapp.NewGetArtifactStatusHandler(deps.ArtifactRepo, deps.ObjectStore),
		listNotebookArtifactsHandler: artifactapp.NewListArtifactsHandler(deps.NotebookRepo, deps.ArtifactRepo),
		cancelArtifactHandler:        artifactapp.NewCancelArtifactHandler(deps.ArtifactRepo, deps.FlowClient, deps.EventBus),
		deleteArtifactHandler:        artifactapp.NewDeleteArtifactHandler(deps.ArtifactRepo, deps.FlowClient, deps.ObjectStore, deps.Cleaner),
		retryArtifactHandler:         artifactapp.NewRetryArtifactHandler(deps.ArtifactRepo, deps.FlowClient, deps.Poller, deps.EventBus),
		updateArtifactHandler:        artifactapp.NewUpdateArtifactHandler(deps.ArtifactRepo),
		convertNoteToSourceHandler: artifactapp.NewConvertNoteToSourceHandler(
			deps.ArtifactRepo,
			deps.SourceRepo,
			deps.NotebookRepo,
			deps.ObjectStore,
			deps.KeyFactory,
			deps.EventBus,
		),
		listStylePreviewsHandler: artifactapp.NewListStylePreviewsHandler(deps.StylePreviewRepo, deps.ObjectStore),
	}
}
