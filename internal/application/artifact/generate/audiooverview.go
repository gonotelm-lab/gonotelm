package generate

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type AudioOverviewGenerator struct {
	deps *ServiceDeps
}

var _ Generator = &AudioOverviewGenerator{}

func (a *AudioOverviewGenerator) Generate(ctx context.Context, req *Request) (*Response, error) {
	return nil, errors.ErrInner.Msg("audio overview generator not implemented")
}
