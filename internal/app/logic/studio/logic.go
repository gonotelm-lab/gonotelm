package studio

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cloudwego/eino/components/document"
	bizartifact "github.com/gonotelm-lab/gonotelm/internal/app/biz/artifact"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/chunker/recursive"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/token"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	"golang.org/x/sync/errgroup"
)

type Logic struct {
	ctx context.Context

	sourceBiz   *bizsource.Biz
	notebookBiz *biznotebook.Biz
	artifactBiz *bizartifact.Biz

	objectStorage storage.Storage
	llmGateway    *gateway.Gateway
	splitter      document.Transformer

	loop *taskLoop
}

func MustNewLogic(
	ctx context.Context,
	objectStorage storage.Storage,
	sourceBiz *bizsource.Biz,
	notebookBiz *biznotebook.Biz,
	artifactBiz *bizartifact.Biz,
	llmGateway *gateway.Gateway,
) *Logic {
	splitter, err := recursive.NewSplitter(context.TODO(), &recursive.Config{
		ChunkSize: constants.MindmapMaxOnceToken,
		LenFunc:   token.Estimate,
	})
	if err != nil {
		panic(err)
	}

	l := &Logic{
		ctx:           ctx,
		objectStorage: objectStorage,
		sourceBiz:     sourceBiz,
		notebookBiz:   notebookBiz,
		artifactBiz:   artifactBiz,
		llmGateway:    llmGateway,
		splitter:      splitter,
	}

	// start background work
	l.initBackgroundWorks()

	return l
}

func (l *Logic) initBackgroundWorks() {
	dispatcher := newTaskDispatcher()
	dispatcher.register(model.ArtifactKindMindmap, &mindmapGenerator{l: l})

	cfg := conf.Global().Logic.Studio.TaskConfig

	l.loop = newTaskLoop(l.ctx, taskLoopConfig{
		numClaimers:        cfg.NumClaimers,
		scanInterval:       cfg.ScanInterval,
		numOfWorkGroup:     cfg.NumOfWorkGroup,
		numWorkersPerGroup: cfg.NumWorkersPerGroup,
	}, l.artifactBiz, dispatcher)
	l.loop.start()
}

func (l *Logic) Close(ctx context.Context) {
	l.loop.stop()
	l.loop.wait()
}

type GenerateArtifactParams struct {
	NotebookId uuid.UUID
	Kind       model.ArtifactKind
	SourceIds  []uuid.UUID
}

func (l *Logic) GenerateArtifact(
	ctx context.Context,
	params *GenerateArtifactParams,
) (uuid.UUID, error) {
	// check notebook and user
	notebook, err := l.helpGetNotebook(ctx, params.NotebookId)
	if err != nil {
		return uuid.EmptyUUID(), err
	}

	userId := pkgcontext.GetUserId(ctx)
	if notebook.OwnerId != userId {
		return uuid.EmptyUUID(), errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", params.NotebookId)
	}

	switch params.Kind {
	case model.ArtifactKindMindmap:
		return l.generateMindmapTask(ctx, &generateMindmapTaskParams{
			NotebookId: params.NotebookId,
			SourceIds:  params.SourceIds,
		})
	default:
		return uuid.EmptyUUID(), errors.ErrParams.Msgf("unsupported artifact kind: %s", params.Kind)
	}
}

func (l *Logic) GetArtifactTaskStatus(
	ctx context.Context,
	taskId uuid.UUID,
) (model.ArtifactStatus, error) {
	status, err := l.artifactBiz.GetTaskStatus(ctx, taskId)
	if err != nil {
		return "", errors.WithMessage(err, "get artifact task status failed")
	}

	return status, nil
}

func (l *Logic) GetArtifactTask(
	ctx context.Context,
	taskId uuid.UUID,
) (*Artifact, error) {
	task, err := l.artifactBiz.GetTask(ctx, taskId)
	if err != nil {
		if errors.Is(err, bizartifact.ErrTaskNotFound) {
			return nil, errors.ErrParams.Msgf("task not found, task_id=%s", taskId)
		}

		return nil, errors.WithMessage(err, "get artifact task failed")
	}

	artifact, err := NewArtifact(task)
	if err != nil {
		return nil, errors.WithMessage(err, "new artifact from task failed")
	}

	if task.Status.Completed() {
		if artifact.ResultKind.Storage() {
			// 补充content url
			resp, err := l.objectStorage.PresignedGetObject(ctx,
				&storage.PresignedGetObjectRequest{
					Key:         artifact.ContentKey,
					ContentType: artifact.ContentType,
				})
			if err != nil {
				return nil, errors.WithMessage(err, "get content url failed")
			}
			artifact.ContentUrl = resp.Url
		}
	}

	return artifact, nil
}

type ListNotebookArtifactsParams struct {
	NotebookId uuid.UUID
	Limit      int
	Offset     int
}

type ListNotebookArtifactsResult struct {
	Artifacts []*Artifact
	HasMore   bool
}

func (l *Logic) ListNotebookArtifacts(
	ctx context.Context,
	params *ListNotebookArtifactsParams,
) (*ListNotebookArtifactsResult, error) {
	_, err := l.helpGetNotebook(ctx, params.NotebookId)
	if err != nil {
		return nil, err
	}

	fetchLimit := params.Limit + 1 // for has_more check
	tasks, err := l.artifactBiz.ListTasksByNotebook(ctx,
		&bizartifact.ListTasksByNotebookQuery{
			NotebookId: params.NotebookId,
			Limit:      fetchLimit,
			Offset:     params.Offset,
		},
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "list notebook artifact tasks failed, notebook_id=%s", params.NotebookId)
	}

	hasMore := len(tasks) > params.Limit
	if hasMore {
		tasks = tasks[:params.Limit]
	}

	artifacts := make([]*Artifact, 0, len(tasks))
	for _, task := range tasks {
		artifact, err := NewArtifact(task)
		if err != nil {
			return nil, errors.WithMessagef(err, "new artifact from task failed, task_id=%s", task.Id)
		}

		if task.Status.Completed() && artifact.ResultKind.Storage() {
			resp, err := l.objectStorage.PresignedGetObject(ctx,
				&storage.PresignedGetObjectRequest{
					Key:         artifact.ContentKey,
					ContentType: artifact.ContentType,
				},
			)
			if err != nil {
				return nil, errors.WithMessagef(err, "get content url failed, task_id=%s", task.Id)
			}
			artifact.ContentUrl = resp.Url
		}

		artifacts = append(artifacts, artifact)
	}

	return &ListNotebookArtifactsResult{
		Artifacts: artifacts,
		HasMore:   hasMore,
	}, nil
}

func (l *Logic) DeleteArtifact(
	ctx context.Context,
	taskId uuid.UUID,
) error {
	task, err := l.artifactBiz.GetTask(ctx, taskId)
	if err != nil {
		return errors.WithMessage(err, "get artifact task failed")
	}
	if task.Status.Running() {
		return errors.ErrParams.Msgf("cannot delete running artifact task, task_id=%s", taskId)
	}

	err = l.artifactBiz.DeleteNotRunningTask(ctx, taskId)
	if err != nil {
		return errors.WithMessage(err, "delete artifact task failed")
	}

	return nil
}

// 重试任务 只有失败的任务才可以重试
func (l *Logic) RetryArtifactTask(
	ctx context.Context,
	taskId uuid.UUID,
) error {
	task, err := l.artifactBiz.GetTask(ctx, taskId)
	if err != nil {
		return errors.WithMessage(err, "get artifact task failed")
	}

	if !task.Status.Failed() {
		return errors.ErrParams.Msgf("can not retry non-failed task, task_id=%s", taskId)
	}

	err = l.artifactBiz.RetryFailedTask(ctx, taskId)
	if err != nil {
		return errors.WithMessage(err, "retry artifact task failed")
	}

	return nil
}

func (l *Logic) CancelArtifactTask(
	ctx context.Context,
	taskId uuid.UUID,
) error {
	task, err := l.artifactBiz.GetTask(ctx, taskId)
	if err != nil {
		return errors.WithMessage(err, "get artifact task failed")
	}

	if !task.Status.Running() {
		return errors.ErrParams.Msgf("can not cancel non-running task, task_id=%s", taskId)
	}

	err = l.artifactBiz.CancelRunningTask(ctx, taskId)
	if err != nil {
		return errors.WithMessage(err, "cancel artifact task failed")
	}

	return nil
}

// 权限校验 检查任务是否属于当前用户
func (l *Logic) CheckArtifactTaskUserId(ctx context.Context, taskId uuid.UUID) error {
	userId := pkgcontext.GetUserId(ctx)
	taskUserId, err := l.artifactBiz.GetArtifactTaskUser(ctx, taskId)
	if err != nil {
		return errors.WithMessage(err, "get artifact task user failed")
	}

	if taskUserId != userId {
		return errors.ErrPermission.Msgf("artifact task access denied, task_id=%s", taskId)
	}

	return nil
}

func (l *Logic) helpGetSourcesParsedContent(
	ctx context.Context,
	sources []*model.DecodedSource,
) (map[uuid.UUID]string, error) {
	var (
		mu       sync.Mutex
		contents map[uuid.UUID]string = make(map[uuid.UUID]string)
	)

	eg, wctx := errgroup.WithContext(ctx)
	for _, source := range sources {
		if source.ParsedContent == nil {
			slog.WarnContext(ctx, "source parsed content is nil", "source_id", source.Id)
			continue
		}

		if source.ParsedContent.StoreKey == "" {
			slog.WarnContext(ctx, "source parsed content store key is empty", "source_id", source.Id)
			continue
		}

		eg.Go(safe.Do(ctx, func() error {
			parsedContent, err := l.objectStorage.GetObject(wctx,
				&storage.GetObjectRequest{
					Key: source.ParsedContent.StoreKey,
				})
			if err != nil {
				return errors.WithMessage(err, "get parsed content failed")
			}

			mu.Lock()
			contents[source.Id] = pkgstring.FromBytes(parsedContent.Body)
			mu.Unlock()

			return nil
		}))
	}
	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	return contents, nil
}

func (l *Logic) helpGetNotebook(ctx context.Context, notebookId uuid.UUID) (*model.Notebook, error) {
	notebook, err := l.notebookBiz.GetNotebook(ctx, notebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return nil, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", notebookId)
		}
		return nil, errors.WithMessage(err, "get notebook failed")
	}

	return notebook, nil
}

func (l *Logic) generateMindmapTask(
	ctx context.Context,
	params *generateMindmapTaskParams,
) (uuid.UUID, error) {
	userId := pkgcontext.GetUserId(ctx)
	payload, err := sonic.Marshal(params)
	if err != nil {
		return uuid.EmptyUUID(), errors.Wrapf(errors.ErrSerde, "marshal mindmap params err=%v", err)
	}

	taskId, err := l.artifactBiz.CreateTask(ctx, &bizartifact.CreateTaskCommand{
		NotebookId: params.NotebookId,
		Kind:       model.ArtifactKindMindmap,
		UserId:     userId,
		Payload:    payload,
	})
	if err != nil {
		return uuid.EmptyUUID(), errors.WithMessagef(err,
			"create mindmap task failed, notebook_id=%s", params.NotebookId)
	}

	return taskId, nil
}
