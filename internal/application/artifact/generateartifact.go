package artifact

import (
	"context"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GenerateRequest struct {
	NotebookId    valobj.Id
	Kind          artifactentity.Kind
	SourceIds     []valobj.Id
	InfoGraphic   *artifactentity.InfoGraphicPayload
	AudioOverview *artifactentity.AudioOverviewPayload
}

type GenerateResponse struct {
	ArtifactId valobj.Id
}

type GenerateArtifactHandler struct {
	repo     artifactrepo.Repository
	flow     flow.TaskClient
	notebook notebookrepo.Repository
	poller   Poller
	eventBus eventbus.EventBus
}

func NewGenerateArtifactHandler(
	repo artifactrepo.Repository,
	flowc flow.TaskClient,
	notebook notebookrepo.Repository,
	poller Poller,
	eventBus eventbus.EventBus,
) *GenerateArtifactHandler {
	return &GenerateArtifactHandler{repo: repo, flow: flowc, notebook: notebook, poller: poller, eventBus: eventBus}
}

func (h *GenerateArtifactHandler) Handle(ctx context.Context, cmd *GenerateRequest) (*GenerateResponse, error) {
	userId := pkgcontext.GetUserId(ctx)

	nb, err := h.notebook.FindById(ctx, cmd.NotebookId)
	if err != nil {
		return nil, err
	}
	if nb.OwnerId != userId {
		return nil, errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", cmd.NotebookId)
	}

	payload, err := buildPayload(cmd)
	if err != nil {
		return nil, err
	}

	artifact, err := artifactentity.NewArtifact(cmd.NotebookId, userId, cmd.Kind, payload)
	if err != nil {
		return nil, err
	}

	payloadBytes, err := sonic.Marshal(payload)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal generate payload err=%v", err)
	}

	flowTaskId, err := h.flow.Submit(ctx, taskTypeFor(cmd.Kind), payloadBytes)
	if err != nil {
		return nil, errors.WithMessage(err, "submit artifact task to flow failed")
	}

	artifact.BindFlowTaskId(flowTaskId)

	if err := h.repo.Save(ctx, artifact); err != nil {
		return nil, errors.WithMessage(err, "save artifact failed")
	}

	for _, evt := range artifact.PullEvents() {
		if err := h.eventBus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish artifact event failed", "artifact_id", artifact.Id, "err", err)
		}
	}

	if h.poller != nil {
		go h.poller.PollOne(context.WithoutCancel(ctx), artifact.Id)
	}

	return &GenerateResponse{ArtifactId: artifact.Id}, nil
}

func buildPayload(req *GenerateRequest) (artifactentity.Payload, error) {
	switch req.Kind {
	case artifactentity.KindMindmap:
		return &artifactentity.MindmapPayload{NotebookId: req.NotebookId, SourceIds: req.SourceIds}, nil
	case artifactentity.KindReport:
		return &artifactentity.ReportPayload{NotebookId: req.NotebookId, SourceIds: req.SourceIds}, nil
	case artifactentity.KindInfoGraphic:
		if req.InfoGraphic == nil {
			return nil, errors.ErrParams.Msgf("info_graphic payload required")
		}
		req.InfoGraphic.NotebookId = req.NotebookId
		req.InfoGraphic.SourceIds = req.SourceIds
		return req.InfoGraphic, nil
	case artifactentity.KindAudioOverview:
		if req.AudioOverview == nil {
			return nil, errors.ErrParams.Msgf("audio_overview payload required")
		}
		req.AudioOverview.NotebookId = req.NotebookId
		req.AudioOverview.SourceIds = req.SourceIds
		return req.AudioOverview, nil
	}
	return nil, errors.ErrParams.Msgf("unsupported artifact kind: %s", req.Kind)
}

func taskTypeFor(kind artifactentity.Kind) string {
	return "artifact." + kind.String()
}
