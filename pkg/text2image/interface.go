package text2image

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/pkg/text2image/schema"
)

type Generator interface {
	Generate(ctx context.Context, req *schema.Request, opts ...Option) (*schema.Response, error)
}
