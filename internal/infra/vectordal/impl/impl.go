package impl

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/impl/milvus"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"

	"google.golang.org/grpc"
)

type Type string

const (
	Milvus Type = "milvus"
)

func New(cfg *Config) (*vectordal.DAL, error) {
	ctx := context.Background()
	switch cfg.Type {
	case Milvus:
		grpcOpts := append(milvusclient.DefaultGrpcOpts, grpc.WithTimeout(cfg.Milvus.DialTimeout))
		cli, err := milvusclient.New(ctx,
			&milvusclient.ClientConfig{
				Address:     cfg.Milvus.Addr,
				Username:    cfg.Milvus.Username,
				Password:    cfg.Milvus.Password,
				DBName:      cfg.Milvus.DBName,
				DialOptions: grpcOpts,
			})
		if err != nil {
			return nil, fmt.Errorf("init milvus client failed: %w", err)
		}
		sourceDocStore, err := milvus.NewSourceDocStoreImpl(cli)
		if err != nil {
			return nil, fmt.Errorf("init milvus source doc store failed: %w", err)
		}

		return &vectordal.DAL{
			Closer: misc.CloserFunc(func(ctx context.Context) error {
				return cli.Close(ctx)
			}),
			SourceDocStore: sourceDocStore,
		}, nil
	}

	return nil, fmt.Errorf("impl type %s is not supported", cfg.Type)
}
