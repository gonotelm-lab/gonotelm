package notelm

import (
	stdcontext "context"
	"sync"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
	chatsuggest "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	authapi "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/auth"
	chatapi "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/chat"
	notebookapi "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	sourceapi "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/source"
	studioapi "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/studio"
	userapi "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/user"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/webhook"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
)

// ServerDeps 是 HTTP 层唯一的注入面：deps 里的字段按功能域裁剪后传给各子包。
type ServerDeps struct {
	RootCtx stdcontext.Context

	NotebookRepo           notebookrepo.Repository
	SourceRepo             sourcerepo.Repository
	ObjectStore            adapter.ObjectStore
	KeyFactory             adapter.StoreKeyFactory
	SourceDocRepo          sourcerepo.SourceDocRepository
	ArtifactRepo           artifactrepo.Repository
	StylePreviewRepo       artifactrepo.StylePreviewRepository
	ChatRepo               chatrepo.ChatRepository
	ChatMessageRepo        chatrepo.MessageRepository
	ChatContextMessageRepo chatrepo.ContextMessageRepository
	ChatStreamTaskRepo     chatrepo.StreamTaskRepository
	ChatSuggestionRepo     chatrepo.SuggestionRepository
	ProviderRegistry       identrepo.ProviderRegistry
	OAuthStateRepo         identrepo.OAuthStateRepository
	UserRepo               identrepo.UserRepository
	UserSessionRepo        identrepo.UserSessionRepository
	UserService            *identityservice.UserService
	SignupService          *identityservice.SignupService
	PasswordResetService   *identityservice.PasswordResetService
	VerifyMailer           identrepo.EmailVerificationMailer
	ResetMailer            identrepo.PasswordResetMailer

	ChatSuggestService *chatsuggest.Service
	EventBus           eventbus.Publisher
	WaitGroup          *sync.WaitGroup
	LLMGateway         *llmchat.Gateway
	DistLock           adapter.DistributedLock
	Summarizer         adapter.Summarizer

	FlowClient flow.TaskClient
	Poller     artifactapp.Poller

	Cleaner *cleanup.Cleaner

	TitleMaker adapter.TitleMaker
}

// Server HTTP 服务本体：只持有各功能域子 Server
type Server struct {
	h *server.Hertz

	auth     *authapi.Server
	notebook *notebookapi.Server
	source   *sourceapi.Server
	chat     *chatapi.Server
	studio   *studioapi.Server
	user     *userapi.Server

	webhook *webhook.Server
}

func NewServer(
	deps ServerDeps,
) *Server {
	hzOpts := []config.Option{
		server.WithCustomBinder(http.NewCanonicalBinder()),
		server.WithHostPorts(conf.NotelmGlobal().Api.HostPort()),
		server.WithExitWaitTime(conf.NotelmGlobal().Api.ExitWaitTimeout),
		server.WithDisablePrintRoute(true),
	}
	hz := server.New(hzOpts...)
	hz.Use(
		middleware.Tracing("notelm"),
		middleware.Recovery(),
		middleware.CORS(middleware.CORSConfig{
			AllowOrigins: conf.NotelmGlobal().Cors.AllowOrigins,
			AllowHeaders: []string{schema.CSRFHeaderName},
		}),
		middleware.Logging(middleware.WithLogAllError(conf.NotelmGlobal().IsDev())),
	)

	s := &Server{
		h: hz,

		auth: authapi.NewServer(authapi.Deps{
			ProviderRegistry:        deps.ProviderRegistry,
			OAuthStateRepo:          deps.OAuthStateRepo,
			UserService:             deps.UserService,
			SignupService:           deps.SignupService,
			PasswordResetService:    deps.PasswordResetService,
			UserSessionRepo:         deps.UserSessionRepo,
			EmailVerificationMailer: deps.VerifyMailer,
			PasswordResetMailer:     deps.ResetMailer,
		}),

		notebook: notebookapi.NewServer(notebookapi.Deps{
			NotebookRepo: deps.NotebookRepo,
			SourceRepo:   deps.SourceRepo,
			EventBus:     deps.EventBus,
			DistLock:     deps.DistLock,
			Summarizer:   deps.Summarizer,
			Cleaner:      deps.Cleaner,
		}),

		source: sourceapi.NewServer(sourceapi.Deps{
			SourceRepo:    deps.SourceRepo,
			NotebookRepo:  deps.NotebookRepo,
			ObjectStore:   deps.ObjectStore,
			KeyFactory:    deps.KeyFactory,
			SourceDocRepo: deps.SourceDocRepo,
			EventBus:      deps.EventBus,
			Cleaner:       deps.Cleaner,
		}),

		chat: chatapi.NewServer(chatapi.Deps{
			WaitGroup:              deps.WaitGroup,
			DistLock:               deps.DistLock,
			NotebookRepo:           deps.NotebookRepo,
			ChatRepo:               deps.ChatRepo,
			ChatMessageRepo:        deps.ChatMessageRepo,
			ChatContextMessageRepo: deps.ChatContextMessageRepo,
			ChatStreamTaskRepo:     deps.ChatStreamTaskRepo,
			ChatSuggestionRepo:     deps.ChatSuggestionRepo,
			SourceRepo:             deps.SourceRepo,
			ObjectStore:            deps.ObjectStore,
			SourceDocRepo:          deps.SourceDocRepo,
			LLMGateway:             deps.LLMGateway,
			EventBus:               deps.EventBus,
			ChatSuggestService:     deps.ChatSuggestService,
		}),

		studio: studioapi.NewServer(studioapi.Deps{
			WaitGroup:        deps.WaitGroup,
			ArtifactRepo:     deps.ArtifactRepo,
			StylePreviewRepo: deps.StylePreviewRepo,
			NotebookRepo:     deps.NotebookRepo,
			SourceRepo:       deps.SourceRepo,
			ChatRepo:         deps.ChatRepo,
			ChatMessageRepo:  deps.ChatMessageRepo,
			FlowClient:       deps.FlowClient,
			Poller:           deps.Poller,
			EventBus:         deps.EventBus,
			TitleMaker:       deps.TitleMaker,
			ObjectStore:      deps.ObjectStore,
			KeyFactory:       deps.KeyFactory,
			Cleaner:          deps.Cleaner,
		}),

		user: userapi.NewServer(userapi.Deps{
			UserRepo:    deps.UserRepo,
			UserService: deps.UserService,
			ObjectStore: deps.ObjectStore,
			KeyFactory:  deps.KeyFactory,
		}),
	}

	if deps.Cleaner != nil && deps.Cleaner.Enabled() {
		s.webhook = webhook.NewServer(deps.Cleaner)
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {
	if s.webhook != nil {
		s.webhook.RegisterRoutes(s.h)
	}

	v1Group := s.h.Group("/api/v1", s.csrfMiddleware())
	{
		v1AuthedGroup := v1Group.Group("/", s.auth.Authenticate())
		{
			s.notebook.RegisterRoutes(v1AuthedGroup)
			s.source.RegisterRoutes(v1AuthedGroup)
			s.chat.RegisterRoutes(v1AuthedGroup)
			s.studio.RegisterRoutes(v1AuthedGroup)
			s.user.RegisterRoutes(v1AuthedGroup)
			s.auth.RegisterAuthedRoutes(v1AuthedGroup)
		}

		s.auth.RegisterPublicRoutes(v1Group)
	}
}

func (s *Server) Hertz() *server.Hertz { return s.h }

func (s *Server) Run() {
	s.h.Spin()
}
