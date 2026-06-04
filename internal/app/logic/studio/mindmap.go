package studio

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/artifact"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/token"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einoschema "github.com/cloudwego/eino/schema"
	"golang.org/x/sync/errgroup"
)

const (
	mindmapAbstractMode = "abstract"
)

type mindmapCreator struct {
	l *Logic
}

var _ taskHandler = &mindmapCreator{}

func (c *mindmapCreator) handle(ctx context.Context, task *model.ArtifactTask) (*taskHandleResult, error) {
	var params generateMindmapTaskParams
	err := sonic.Unmarshal(task.Payload, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal generate mindmap task params err=%v", err)
	}

	mindmap, err := c.l.createMindmap(ctx, &createMindmapParams{
		NotebookId: params.NotebookId,
		SourceIds:  params.SourceIds,
	})
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "create mindmap failed, err=%v", err)
	}

	return &taskHandleResult{
		result:     pkgstring.AsBytes(mindmap),
		resultKind: model.ArtifactResultKindInline,
	}, nil
}

type generateMindmapTaskParams struct {
	NotebookId uuid.UUID   `json:"notebook_id"`
	SourceIds  []uuid.UUID `json:"source_ids"`
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

	taskId, err := l.artifactBiz.CreateTask(ctx, &artifact.CreateTaskCommand{
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

type createMindmapParams struct {
	NotebookId uuid.UUID
	SourceIds  []uuid.UUID
}

func (l *Logic) createMindmap(ctx context.Context, params *createMindmapParams) (string, error) {
	// check notebook
	notebook, err := l.helpGetNotebook(ctx, params.NotebookId)
	if err != nil {
		return "", errors.WithMessage(err, "get notebook failed")
	}

	// check source ids ready
	sources, err := l.sourceBiz.BatchGetDecodedSources(
		ctx,
		notebook.Id,
		params.SourceIds,
	)
	if err != nil {
		return "", errors.WithMessage(err, "batch get decoded sources failed")
	}

	lenSources := len(sources)
	if lenSources == 0 {
		return "", errors.ErrParams.Msgf(
			"no sources found, notebook_id=%s, source_ids=%v",
			notebook.Id, params.SourceIds,
		)
	}

	parsedContents, err := l.helpGetSourcesParsedContent(ctx, sources)
	if err != nil {
		return "", errors.WithMessagef(err,
			"get sources parsed content failed, notebook_id=%s",
			notebook.Id,
		)
	}
	if len(parsedContents) == 0 {
		return "", errors.ErrParams.Msg("empty source contents")
	}

	// 思维导图是对所有选中来源的整体探索 所以还是需要全量给到LLM进行处理
	totalTokens := 0
	tokensCounts := make(map[uuid.UUID]int)
	for id, content := range parsedContents {
		count := token.Estimate(content)
		totalTokens += count
		tokensCounts[id] = count
	}

	if totalTokens <= constants.MindmapMaxOnceToken {
		return l.oneshotCreateMindmap(ctx, params.NotebookId, parsedContents, "")
	}

	return l.twoshotCreateMindmap(ctx, params.NotebookId, parsedContents, tokensCounts)
}

func (l *Logic) oneshotCreateMindmap(
	ctx context.Context,
	notebookId uuid.UUID,
	contents map[uuid.UUID]string,
	mode string,
) (string, error) {
	tmps := make([]string, 0, len(contents))
	for _, v := range contents {
		tmps = append(tmps, v)
	}

	var (
		msg  *einoschema.Message
		err  error
		lang = ""
	)

	if mode == mindmapAbstractMode {
		msg, err = prompts.StudioMindmapAbstractMessage(ctx, tmps, lang)
	} else {
		msg, err = prompts.StudioMindmapContentMessage(ctx, tmps, lang)
	}
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner,
			"failed to get mindmap template message, mode=%s, err=%v", mode, err)
	}

	model, err := l.llmGateway.GetProvider(
		conf.Global().Logic.Studio.Mindmap.ModelProvider,
	)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "failed to get studio mindmap model, err=%v", err)
	}

	msgs := pkgslices.FromSingle(msg)

	llmOption := llmchat.BuildLLMModelOption(conf.Global().Logic.Studio.Mindmap.Model)
	const retryTimes = 3
	for idx := range retryTimes {
		llmResp, err := model.Generate(ctx, msgs, llmOption)
		if err != nil {
			return "", errors.Wrapf(errors.ErrLLM,
				"failed to generate studio mindmap, retry=%d, err=%v",
				idx, err,
			)
		}

		content := llmResp.Content
		if !prompts.CheckStudioMindmapResult(content) {
			// 给多一次机会
			compensateMsg := &einoschema.Message{
				Role: einoschema.User,
				Content: fmt.Sprintf(
					"你刚才生成的思维导图不符合要求的格式，请重新输出，这个是你当前生成的结果:\n%s\n\n请严格按照格式要求重新输出",
					content,
				),
			}
			msgs = append(msgs, compensateMsg)

			slog.WarnContext(ctx, "studio mindmap result not valid, compensating",
				slog.String("notebook_id", notebookId.String()))
			continue
		}

		return content, nil
	}

	return "", errors.Wrap(errors.ErrLLM, "failed to generate studio mindmap")
}

func (l *Logic) twoshotCreateMindmap(
	ctx context.Context,
	notebookId uuid.UUID,
	contents map[uuid.UUID]string,
	tokensCounts map[uuid.UUID]int,
) (string, error) {
	type contentInfo struct {
		id         uuid.UUID
		content    string
		tokenCount int
	}

	contentInfos := make([]contentInfo, 0, len(contents))
	for key, val := range contents {
		contentInfos = append(contentInfos, contentInfo{
			id:         key,
			content:    val,
			tokenCount: tokensCounts[key],
		})
	}
	// sort by token count asc
	sort.Slice(contentInfos, func(i, j int) bool {
		return contentInfos[i].tokenCount < contentInfos[j].tokenCount
	})

	type contentBatch struct {
		contents    []contentInfo
		totalTokens int
	}

	// 将contentInfos分成多个batch
	batches := make([]*contentBatch, 1, len(contentInfos))
	batches[0] = &contentBatch{
		contents:    []contentInfo{contentInfos[0]},
		totalTokens: contentInfos[0].tokenCount,
	}
	currentTokens := contentInfos[0].tokenCount
	for idx := 1; idx < len(contentInfos); idx++ {
		info := contentInfos[idx]
		newTokens := currentTokens + info.tokenCount
		if newTokens > constants.MindmapMaxOnceToken {
			// create a new batch
			batches = append(batches, &contentBatch{
				contents:    []contentInfo{info},
				totalTokens: info.tokenCount,
			})
			currentTokens = info.tokenCount
		} else {
			// join current batch
			batches[len(batches)-1].contents = append(batches[len(batches)-1].contents, info)
			batches[len(batches)-1].totalTokens += info.tokenCount
			currentTokens = newTokens
		}
	}

	type batchContent struct {
		contents []string
	}

	finalBatches := make([]*batchContent, 0, len(batches))
	for _, batch := range batches {
		if batch.totalTokens > constants.MindmapMaxOnceToken {
			// 检查每个batch是否超限 如果超限再拆开
			ccs := strings.Builder{}
			for _, c := range batch.contents {
				ccs.WriteString(c.content)
				ccs.WriteByte('\n')
			}
			splits, err := l.splitter.Transform(ctx,
				pkgslices.FromSingle(&einoschema.Document{
					Content: ccs.String(),
				}))
			if err != nil {
				return "", errors.WithMessagef(err, "split content failed, id=%s", batch.contents[0].id)
			}

			for _, split := range splits {
				finalBatches = append(finalBatches, &batchContent{contents: []string{split.Content}})
			}
		} else {
			contents := make([]string, 0, len(batch.contents))
			for _, s := range batch.contents {
				contents = append(contents, s.content)
			}
			finalBatches = append(finalBatches, &batchContent{contents: contents})
		}
	}

	// step1
	eg, ctx2 := errgroup.WithContext(ctx)
	resps := make([]string, len(finalBatches))
	for idx, batch := range finalBatches {
		eg.Go(safe.Do(ctx2, func() error {
			resp, err := prompts.StudioMindmapContentMessage(ctx, batch.contents, "")
			if err != nil {
				return errors.Wrapf(errors.ErrLLM, "generate mindmap content failed, err=%v", err)
			}

			resps[idx] = resp.Content
			return nil
		}))
	}
	err := eg.Wait()
	if err != nil {
		return "", errors.WithMessage(err, "split contents failed")
	}

	abstractContents := make(map[uuid.UUID]string, len(resps))
	for _, r := range resps {
		if r != "" {
			abstractContents[uuid.NewV4()] = r
		}
	}
	// step2
	mindmap, err := l.oneshotCreateMindmap(
		ctx,
		notebookId,
		abstractContents,
		mindmapAbstractMode,
	)
	if err != nil {
		return "", errors.WithMessage(err, "generate abstract mindmap failed")
	}

	return mindmap, nil
}
