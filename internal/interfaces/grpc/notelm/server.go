package notelm

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	userapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/user"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	devident "github.com/gonotelm-lab/gonotelm/internal/interfaces/grpc/notelm/dev/identity"
	grpcmw "github.com/gonotelm-lab/gonotelm/pkg/grpc/middleware"

	identityv1 "github.com/gonotelm-lab/apis-go-client/apps/notelm/dev/identity/v1"

	"google.golang.org/grpc"
)

type Server struct {
	cfg    conf.GrpcConfig
	server *grpc.Server
	lis    net.Listener
}

type Deps struct {
	GetUser       *userapp.GetUserHandler
	SetUserStatus *userapp.SetUserStatusHandler
}

func NewServer(cfg conf.GrpcConfig, deps Deps) (*Server, error) {
	lis, err := net.Listen("tcp", cfg.HostPort())
	if err != nil {
		return nil, fmt.Errorf("listen grpc %s: %w", cfg.HostPort(), err)
	}

	gs := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcmw.UnaryServerInterceptors()...),
	)

	identityv1.RegisterIdentityServiceServer(gs, devident.NewServer(devident.Deps{
		GetUser:       deps.GetUser,
		SetUserStatus: deps.SetUserStatus,
	}))

	return &Server{
		cfg:    cfg,
		server: gs,
		lis:    lis,
	}, nil
}

func (s *Server) Start() {
	addr := s.lis.Addr().String()
	slog.Info("starting grpc server", slog.String("addr", addr))
	go func() {
		if err := s.server.Serve(s.lis); err != nil {
			slog.Error("grpc server stopped", slog.String("addr", addr), slog.Any("err", err))
		}
	}()
}

func (s *Server) Close() error {
	done := make(chan struct{})
	go func() {
		slog.Warn("Closing grpc server")
		s.server.GracefulStop()
		close(done)
	}()

	if s.cfg.ExitWaitTimeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ExitWaitTimeout)
		defer cancel()
		select {
		case <-done:
		case <-ctx.Done():
			s.server.Stop()
		}
		return nil
	}

	<-done
	return nil
}
