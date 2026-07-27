package artifact

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
)

type GenerateRequest struct {
	NotebookId    valobj.Id
	Kind          artifactentity.Kind
	SourceIds     []valobj.Id
	Mindmap       *artifactentity.MindmapPayload
	Report        *artifactentity.ReportPayload
	InfoGraphic   *artifactentity.InfoGraphicPayload
	AudioOverview *artifactentity.AudioOverviewPayload
	Flashcard     *artifactentity.FlashcardPayload
	Quiz          *artifactentity.QuizPayload
	DataTable     *artifactentity.DataTablePayload
	Note          *artifactentity.NotePayload
}

type GenerateResponse struct {
	ArtifactId valobj.Id
}

type GenerateArtifactHandler struct {
	wg           *sync.WaitGroup
	artifactRepo artifactrepo.Repository
	chatMsgRepo  chatrepo.MessageRepository
	chatRepo     chatrepo.Repository
	notebookRepo notebookrepo.Repository
	flow         flow.TaskClient
	poller       Poller
	eventBus     eventbus.EventBus
	titleMaker   adapter.TitleMaker
}

func NewGenerateArtifactHandler(
	wg *sync.WaitGroup,
	artifactRepo artifactrepo.Repository,
	notebookRepo notebookrepo.Repository,
	chatRepo chatrepo.Repository,
	chatMsgRepo chatrepo.MessageRepository,
	flowc flow.TaskClient,
	poller Poller,
	eventBus eventbus.EventBus,
	titleMaker adapter.TitleMaker,
) *GenerateArtifactHandler {
	return &GenerateArtifactHandler{
		wg:           wg,
		artifactRepo: artifactRepo,
		notebookRepo: notebookRepo,
		chatRepo:     chatRepo,
		chatMsgRepo:  chatMsgRepo,
		flow:         flowc,
		poller:       poller,
		eventBus:     eventBus,
		titleMaker:   titleMaker,
	}
}

func (h *GenerateArtifactHandler) Handle(ctx context.Context, cmd *GenerateRequest) (*GenerateResponse, error) {
	userId := pkgcontext.GetUserId(ctx)

	nb, err := h.notebookRepo.FindById(ctx, cmd.NotebookId)
	if err != nil {
		return nil, errors.WithMessagef(err, "find notebook by id=%s failed", cmd.NotebookId)
	}
	if nb.OwnerId != userId {
		return nil, errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", cmd.NotebookId)
	}

	// 保存为笔记单独处理 不需要worker处理 直接本地处理即可
	if cmd.Kind == artifactentity.KindNote {
		return h.saveAsNote(ctx, cmd, userId)
	}

	return h.beginArtifactTask(ctx, cmd, userId)
}

func (r *GenerateRequest) buildPayload() (artifactentity.Payload, error) {
	switch r.Kind {
	case artifactentity.KindMindmap:
		if r.Mindmap == nil {
			return &artifactentity.MindmapPayload{
				NotebookId: r.NotebookId,
				SourceIds:  r.SourceIds,
			}, nil
		}
		r.Mindmap.NotebookId = r.NotebookId
		r.Mindmap.SourceIds = r.SourceIds
		return r.Mindmap, nil
	case artifactentity.KindReport:
		if r.Report == nil {
			return &artifactentity.ReportPayload{
				NotebookId: r.NotebookId,
				SourceIds:  r.SourceIds,
			}, nil
		}
		r.Report.NotebookId = r.NotebookId
		r.Report.SourceIds = r.SourceIds
		return r.Report, nil
	case artifactentity.KindInfoGraphic:
		if r.InfoGraphic == nil {
			return nil, errors.ErrParams.Msgf("info_graphic payload required")
		}
		r.InfoGraphic.NotebookId = r.NotebookId
		r.InfoGraphic.SourceIds = r.SourceIds
		return r.InfoGraphic, nil
	case artifactentity.KindAudioOverview:
		if r.AudioOverview == nil {
			return nil, errors.ErrParams.Msgf("audio_overview payload required")
		}
		r.AudioOverview.NotebookId = r.NotebookId
		r.AudioOverview.SourceIds = r.SourceIds
		return r.AudioOverview, nil
	case artifactentity.KindFlashcard:
		if r.Flashcard == nil {
			return &artifactentity.FlashcardPayload{
				NotebookId: r.NotebookId,
				SourceIds:  r.SourceIds,
				Count:      artifactentity.FlashcardCountDefaultValue(),
				Difficulty: artifactentity.FlashcardDifficultyDefault(),
			}, nil
		}
		r.Flashcard.NotebookId = r.NotebookId
		r.Flashcard.SourceIds = r.SourceIds
		if !r.Flashcard.Count.Supported() {
			r.Flashcard.Count = artifactentity.FlashcardCountDefaultValue()
		}
		if !r.Flashcard.Difficulty.Supported() {
			r.Flashcard.Difficulty = artifactentity.FlashcardDifficultyDefault()
		}
		return r.Flashcard, nil
	case artifactentity.KindQuiz:
		if r.Quiz == nil {
			return &artifactentity.QuizPayload{
				NotebookId: r.NotebookId,
				SourceIds:  r.SourceIds,
				Count:      artifactentity.QuizCountDefaultValue(),
				Difficulty: artifactentity.QuizDifficultyDefault(),
			}, nil
		}
		r.Quiz.NotebookId = r.NotebookId
		r.Quiz.SourceIds = r.SourceIds
		if !r.Quiz.Count.Supported() {
			r.Quiz.Count = artifactentity.QuizCountDefaultValue()
		}
		if !r.Quiz.Difficulty.Supported() {
			r.Quiz.Difficulty = artifactentity.QuizDifficultyDefault()
		}
		return r.Quiz, nil
	case artifactentity.KindDataTable:
		if r.DataTable == nil {
			return &artifactentity.DataTablePayload{
				NotebookId: r.NotebookId,
				SourceIds:  r.SourceIds,
			}, nil
		}
		r.DataTable.NotebookId = r.NotebookId
		r.DataTable.SourceIds = r.SourceIds
		return r.DataTable, nil
	}

	return nil, errors.ErrParams.Msgf("unsupported artifact kind: %s", r.Kind)
}

func taskTypeFor(kind artifactentity.Kind) string {
	return "artifact." + kind.String()
}

func (h *GenerateArtifactHandler) beginArtifactTask(
	ctx context.Context,
	cmd *GenerateRequest,
	userId string,
) (*GenerateResponse, error) {
	// 构建所有产物生成的请求payload
	payload, err := cmd.buildPayload()
	if err != nil {
		return nil, err
	}

	artifact, err := artifactentity.NewArtifact(
		cmd.NotebookId,
		userId,
		cmd.Kind,
		payload,
	)
	if err != nil {
		return nil, err
	}

	payloadBytes, err := sonic.Marshal(payload)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal generate payload err=%v", err)
	}

	workerInput := buildWorkerInput(artifact, payloadBytes)
	workerInputBytes, err := sonic.Marshal(workerInput)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal worker input err=%v", err)
	}

	flowTaskId, err := h.flow.Submit(ctx, taskTypeFor(cmd.Kind), workerInputBytes)
	if err != nil {
		return nil, errors.WithMessage(err, "submit artifact task to flow failed")
	}

	artifact.BindFlowTaskId(flowTaskId)

	if err := h.artifactRepo.Save(ctx, artifact); err != nil {
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

// 将对话内容保存为笔记：先落库（running），再异步生成 title 后更新为 completed。
// 不走 flow，前端通过 status 轮询等待终态即可。
func (h *GenerateArtifactHandler) saveAsNote(
	ctx context.Context,
	req *GenerateRequest,
	userId string,
) (*GenerateResponse, error) {
	if req.Note == nil {
		return nil, errors.ErrParams.Msg("note payload is required")
	}

	if req.Note.ChatId.IsZero() || req.Note.MsgId.IsZero() {
		return nil, errors.ErrParams.Msg("chat_id and msg_id are required")
	}

	targetChat, err := h.chatRepo.FindById(ctx, req.Note.ChatId)
	if err != nil {
		return nil, errors.WithMessagef(err, "find chat by id=%s failed", req.Note.ChatId)
	}
	if targetChat.NotebookId != req.NotebookId {
		return nil, errors.ErrPermission.Msg("chat is not in the notebook")
	}
	if targetChat.OwnerId != userId {
		return nil, errors.ErrPermission.Msg("chat is not owned by the user")
	}

	targetMsg, err := h.chatMsgRepo.FindByChatIdMsgId(
		ctx,
		req.Note.ChatId,
		req.Note.MsgId,
	)
	if err != nil {
		return nil, errors.WithMessagef(err,
			"find chat message by chat_id=%s, msg_id=%s failed",
			req.Note.ChatId, req.Note.MsgId)
	}
	if targetMsg.UserId != userId {
		return nil, errors.ErrPermission.Msg("chat message is not owned by the user")
	}

	if len(targetMsg.Fragments) == 0 {
		return nil, errors.ErrParams.Msgf("chat message has no fragments")
	}

	if targetMsg.Role != chatentity.MessageRoleAssistant {
		return nil, errors.ErrParams.Msgf("can not convert non-assistant message to note")
	}

	targetFragment := targetMsg.Fragments[len(targetMsg.Fragments)-1]
	if targetFragment.Type != chatentity.FragmentTypeResponse || targetFragment.Response == nil {
		return nil, errors.ErrParams.Msgf("can not convert message to note")
	}

	targetContent := targetFragment.Response.Content
	if targetContent == nil || targetContent.Text == nil || targetContent.Text.Content() == "" {
		return nil, errors.ErrParams.Msgf("can not convert message to note")
	}

	textContent := targetContent.Text.Content()
	newArtifact, err := artifactentity.NewArtifact(
		req.NotebookId,
		userId,
		artifactentity.KindNote,
		&artifactentity.NotePayload{
			ChatId: req.Note.ChatId,
			MsgId:  req.Note.MsgId,
		},
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "create artifact failed")
	}

	newArtifact.MarkRunning()
	// 这里不需要调用flow 所以随便绑定一个值即可
	newArtifact.BindFlowTaskId(newArtifact.Id.String())
	if err := h.artifactRepo.Save(ctx, newArtifact); err != nil {
		return nil, errors.WithMessagef(err, "save artifact failed")
	}

	taskCtx := context.WithoutCancel(ctx)
	h.wg.Go(func() { h.finalizeNoteTitle(taskCtx, newArtifact.Id, textContent) })

	return &GenerateResponse{ArtifactId: newArtifact.Id}, nil
}

func (h *GenerateArtifactHandler) finalizeNoteTitle(
	ctx context.Context,
	artifactId valobj.Id,
	textContent string,
) {
	targetArtifact, err := h.artifactRepo.FindById(ctx, artifactId)
	if err != nil {
		slog.ErrorContext(ctx, "finalize note: find artifact failed", "artifact_id", artifactId, "err", err)
		return
	}
	if targetArtifact.IsTerminal() {
		return
	}

	title, err := h.titleMaker.MakeTitle(ctx, textContent)
	if err != nil {
		slog.ErrorContext(ctx, "finalize note: make title failed",
			slog.String("artifact_id", artifactId.String()),
			slog.String("err", err.Error()),
		)
		targetArtifact.MarkFailed()
	} else {
		targetArtifact.MarkCompleted(pkgstring.AsBytes(textContent), artifactentity.ResultKindInline, title)
	}

	if err := h.artifactRepo.Save(ctx, targetArtifact); err != nil {
		slog.ErrorContext(ctx, "finalize note: save completed failed",
			slog.String("artifact_id", artifactId.String()),
			slog.String("err", err.Error()),
		)
	}
}
