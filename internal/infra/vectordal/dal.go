package vectordal

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
)

type SourceDocStore interface {
	BatchInsert(ctx context.Context, docs []*schema.SourceDoc) error
	BatchDelete(ctx context.Context, params *schema.SourceDocBatchDeleteParams) error
	Get(ctx context.Context, params *schema.SourceDocGetParams) (*schema.SourceDoc, error)
	Query(ctx context.Context, params *schema.SourceDocQueryParams) ([]*schema.SourceDoc, error)
}

type DAL struct {
	Closer misc.Closer

	SourceDocStore SourceDocStore
}

func (d *DAL) Close(ctx context.Context) error {
	slog.WarnContext(ctx, "closing vector database connections...")
	return d.Closer.Close(ctx)
}
