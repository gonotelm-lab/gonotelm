package identity

import (
	"context"

	userapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/user"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/grpc/notelm/schema"

	identityv1 "github.com/gonotelm-lab/apis-go-client/apps/notelm/dev/identity/v1"
)

type Server struct {
	identityv1.UnimplementedIdentityServiceServer

	getUser       *userapp.GetUserHandler
	setUserStatus *userapp.SetUserStatusHandler
}

type Deps struct {
	GetUser       *userapp.GetUserHandler
	SetUserStatus *userapp.SetUserStatusHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		getUser:       deps.GetUser,
		setUserStatus: deps.SetUserStatus,
	}
}

func (s *Server) GetUser(ctx context.Context, req *identityv1.GetUserRequest) (*identityv1.GetUserResponse, error) {
	view, err := s.getUser.Handle(ctx, &userapp.GetUserQuery{UserId: req.GetId()})
	if err != nil {
		return nil, err
	}

	return &identityv1.GetUserResponse{User: schema.ToProtoUser(view)}, nil
}

func (s *Server) SetUserStatus(
	ctx context.Context,
	req *identityv1.SetUserStatusRequest,
) (*identityv1.SetUserStatusResponse, error) {
	status, err := schema.FromProtoStatus(req.GetStatus())
	if err != nil {
		return nil, err
	}

	view, err := s.setUserStatus.Handle(ctx, &userapp.SetUserStatusCommand{
		UserId: req.GetId(),
		Status: status,
	})
	if err != nil {
		return nil, err
	}

	return &identityv1.SetUserStatusResponse{User: schema.ToProtoUser(view)}, nil
}
