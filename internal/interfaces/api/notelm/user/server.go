package user

import (
	userapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/user"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// Deps 账号资料接口所需依赖，由外层 notelm.ServerDeps 裁剪后传入
type Deps struct {
	UserRepo    identrepo.UserRepository
	UserService *identityservice.UserService
	ObjectStore adapter.ObjectStore
	KeyFactory  adapter.StoreKeyFactory
}

// Server 账号资料接口的处理器集合
type Server struct {
	getMeHandler         *userapp.GetMeHandler
	updateProfileHandler *userapp.UpdateProfileHandler
	updateAvatarHandler  *userapp.UpdateAvatarHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		getMeHandler:         userapp.NewGetMeHandler(deps.UserService, deps.ObjectStore),
		updateProfileHandler: userapp.NewUpdateProfileHandler(deps.UserRepo),
		updateAvatarHandler:  userapp.NewUpdateAvatarHandler(deps.UserRepo, deps.ObjectStore, deps.KeyFactory),
	}
}
