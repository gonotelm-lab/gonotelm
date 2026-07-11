package vectordb

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
)

type SourceDocStore interface {
	BatchInsert(ctx context.Context, docs []*schema.SourceDoc) error
	BatchDelete(ctx context.Context, params *schema.SourceDocBatchDeleteParams) error
	Get(ctx context.Context, params *schema.SourceDocGetParams) (*schema.SourceDoc, error)
	BatchGet(ctx context.Context, params *schema.SourceDocBatchGetParams) ([]*schema.SourceDoc, error)
	Query(ctx context.Context, params *schema.SourceDocQueryParams) ([]*schema.SourceDoc, error)

	List(ctx context.Context, params *schema.SourceDocListParams) ([]*schema.SourceDoc, error)

	ListByChunkPos(ctx context.Context, params *schema.SourceDocListByChunkPosParams) ([]*schema.SourceDoc, error)
}

type DAL struct {
	Closer misc.Closer

	SourceDocStore SourceDocStore
}

func (d *DAL) Close(ctx context.Context) error {
	slog.WarnContext(ctx, "closing vector database connections...")
	return d.Closer.Close(ctx)
}
