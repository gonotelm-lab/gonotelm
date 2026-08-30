package worker

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/audiooverview"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/datatable"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/flashcard"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/infographic"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/mindmap"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/quiz"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/report"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/slides"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func run(ctx context.Context, deps *types.WorkerDeps, req *types.Request) (*types.Response, error) {
	g, scene, err := newGenerator(req.Kind, deps)
	if err != nil {
		return nil, err
	}

	ctx = pkgcontext.WithScene(ctx, scene, req.ArtifactId.String())

	return g.Generate(ctx, req)
}

func newGenerator(kind entity.Kind, deps *types.WorkerDeps) (types.Generator, pkgcontext.SceneType, error) {
	switch kind {
	case entity.KindMindmap:
		return mindmap.New(deps), pkgcontext.StudioMindmapScene, nil
	case entity.KindReport:
		return report.New(deps), pkgcontext.StudioReportScene, nil
	case entity.KindInfoGraphic:
		return infographic.New(deps), pkgcontext.StudioInfographicScene, nil
	case entity.KindAudioOverview:
		return audiooverview.New(deps), pkgcontext.StudioAudioOverviewScene, nil
	case entity.KindFlashcard:
		return flashcard.New(deps), pkgcontext.StudioFlashcardScene, nil
	case entity.KindQuiz:
		return quiz.New(deps), pkgcontext.StudioQuizScene, nil
	case entity.KindDataTable:
		return datatable.New(deps), pkgcontext.StudioDataTableScene, nil
	case entity.KindSlides:
		return slides.New(deps), pkgcontext.StudioSlidesScene, nil
	}
	return nil, pkgcontext.UnknownScene, errors.ErrParams.Msgf("unsupported kind: %s", kind)
}
