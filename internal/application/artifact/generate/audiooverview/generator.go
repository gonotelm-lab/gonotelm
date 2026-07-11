package audiooverview

import (
	"context"

	generatetypes "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type Generator struct {
	deps *generatetypes.ServiceDeps
}

var _ generatetypes.Generator = &Generator{}

func New(deps *generatetypes.ServiceDeps) *Generator {
	return &Generator{deps: deps}
}

func (a *Generator) Generate(ctx context.Context, req *generatetypes.Request) (*generatetypes.Response, error) {
	return nil, errors.ErrInner.Msg("audio overview generator not implemented")
}
