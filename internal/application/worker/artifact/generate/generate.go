package generate

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/audiooverview"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/datatable"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/flashcard"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/infographic"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/mindmap"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/quiz"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/report"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func Run(ctx context.Context, deps *types.ServiceDeps, req *types.Request) (*types.Response, error) {
	g, err := newGenerator(req.Kind, deps)
	if err != nil {
		return nil, err
	}
	return g.Generate(ctx, req)
}

func newGenerator(kind artifactentity.Kind, deps *types.ServiceDeps) (types.Generator, error) {
	switch kind {
	case artifactentity.KindMindmap:
		return mindmap.New(deps), nil
	case artifactentity.KindReport:
		return report.New(deps), nil
	case artifactentity.KindInfoGraphic:
		return infographic.New(deps), nil
	case artifactentity.KindAudioOverview:
		return audiooverview.New(deps), nil
	case artifactentity.KindFlashcard:
		return flashcard.New(deps), nil
	case artifactentity.KindQuiz:
		return quiz.New(deps), nil
	case artifactentity.KindDataTable:
		return datatable.New(deps), nil
	}
	return nil, errors.ErrParams.Msgf("unsupported kind: %s", kind)
}
