package generate

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/audiooverview"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/infographic"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/mindmap"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/report"
	generatetypes "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func Run(ctx context.Context, deps *generatetypes.ServiceDeps, req *generatetypes.Request) (*generatetypes.Response, error) {
	g, err := newGenerator(req.Kind, deps)
	if err != nil {
		return nil, err
	}
	return g.Generate(ctx, req)
}

func newGenerator(kind artifactentity.Kind, deps *generatetypes.ServiceDeps) (generatetypes.Generator, error) {
	switch kind {
	case artifactentity.KindMindmap:
		return mindmap.New(deps), nil
	case artifactentity.KindReport:
		return report.New(deps), nil
	case artifactentity.KindInfoGraphic:
		return infographic.New(deps), nil
	case artifactentity.KindAudioOverview:
		return audiooverview.New(deps), nil
	}
	return nil, errors.ErrParams.Msgf("unsupported kind: %s", kind)
}
