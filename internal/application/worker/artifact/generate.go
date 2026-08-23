package artifact

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
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func Run(ctx context.Context, deps *types.ServiceDeps, req *types.Request) (*types.Response, error) {
	g, scene, err := newGenerator(req.Kind, deps)
	if err != nil {
		return nil, err
	}

	ctx = pkgcontext.WithScene(ctx, scene, req.ArtifactId.String())

	return g.Generate(ctx, req)
}

func newGenerator(kind artifactentity.Kind, deps *types.ServiceDeps) (types.Generator, pkgcontext.SceneType, error) {
	switch kind {
	case artifactentity.KindMindmap:
		return mindmap.New(deps), pkgcontext.StudioMindmapScene, nil
	case artifactentity.KindReport:
		return report.New(deps), pkgcontext.StudioReportScene, nil
	case artifactentity.KindInfoGraphic:
		return infographic.New(deps), pkgcontext.StudioInfographicScene, nil
	case artifactentity.KindAudioOverview:
		return audiooverview.New(deps), pkgcontext.StudioAudioOverviewScene, nil
	case artifactentity.KindFlashcard:
		return flashcard.New(deps), pkgcontext.StudioFlashcardScene, nil
	case artifactentity.KindQuiz:
		return quiz.New(deps), pkgcontext.StudioQuizScene, nil
	case artifactentity.KindDataTable:
		return datatable.New(deps), pkgcontext.StudioDataTableScene, nil
	case artifactentity.KindSlides:
		return slides.New(deps), pkgcontext.StudioSlidesScene, nil
	}
	return nil, pkgcontext.UnknownScene, errors.ErrParams.Msgf("unsupported kind: %s", kind)
}
