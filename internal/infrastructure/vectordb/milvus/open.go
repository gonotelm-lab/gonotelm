package milvus

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"

	"google.golang.org/grpc"
)

func Open(cfg *vectordb.Config) (*vectordb.DAL, error) {
	ctx := context.Background()

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
	sourceDocStore, err := NewSourceDocStoreImpl(cli)
	if err != nil {
		return nil, fmt.Errorf("init milvus source doc store failed: %w", err)
	}

	return &vectordb.DAL{
		Closer: misc.CloserFunc(func(ctx context.Context) error {
			return cli.Close(ctx)
		}),
		SourceDocStore: sourceDocStore,
	}, nil
}
