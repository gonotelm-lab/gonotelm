package studio

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/bytedance/sonic"
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

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"golang.org/x/sync/errgroup"
)

const (
	mindmapAbstractMode = "abstract"
	mindmapContentMode  = "content"
)

type mindmapGenerator struct {
	l *Logic
}

var _ taskHandler = &mindmapGenerator{}

type generateMindmapTaskParams struct {
	*commonTaskParams
}

type mindmapExpectation struct {
	Title   string `json:"title"`
	Mindmap string `json:"mindmap"`
}

func (m *mindmapGenerator) handle(
	ctx context.Context,
	task *model.ArtifactTask,
) (*taskHandleResult, error) {
	var params generateMindmapTaskParams
	err := sonic.Unmarshal(task.Payload, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal generate mindmap task params err=%v", err)
	}

	expect, err := m.generate(ctx, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "create mindmap failed, err=%v", err)
	}

	return &taskHandleResult{
		result:     pkgstring.AsBytes(expect.Mindmap),
		resultKind: model.ArtifactResultKindInline,
		title:      expect.Title,
	}, nil
}

func (m *mindmapGenerator) generate(
	ctx context.Context,
	params *generateMindmapTaskParams,
) (*mindmapExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioMindmapScene)
	// check notebook
	notebook, err := m.l.helpGetNotebook(ctx, params.NotebookId)
	if err != nil {
		return nil, errors.WithMessage(err, "get notebook failed")
	}

	// check source ids ready
	sources, err := m.l.sourceBiz.BatchGetDecodedSources(
		ctx,
		notebook.Id,
		params.SourceIds,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "batch get decoded sources failed")
	}

	lenSources := len(sources)
	if lenSources == 0 {
		return nil, errors.ErrParams.Msgf(
			"no sources found, notebook_id=%s, source_ids=%v",
			notebook.Id, params.SourceIds,
		)
	}

	parsedContents, err := m.l.helpGetSourcesParsedContent(ctx, sources)
	if err != nil {
		return nil, errors.WithMessagef(err,
			"get sources parsed content failed, notebook_id=%s",
			notebook.Id,
		)
	}
	if len(parsedContents) == 0 {
		return nil, errors.ErrParams.Msg("empty source contents")
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
		return m.oneshotCreateMindmap(ctx, params.NotebookId, parsedContents, "")
	}

	return m.twoshotCreateMindmap(ctx, params.NotebookId, parsedContents, tokensCounts)
}

func (m *mindmapGenerator) llmOptions() (
	einomodel.ToolCallingChatModel,
	[]einomodel.Option,
	error,
) {
	var (
		provider = conf.Global().Logic.Studio.Mindmap.ModelProvider
		model    = conf.Global().Logic.Studio.Mindmap.Model
	)
	chatModel, err := m.l.llmGateway.GetProvider(provider)
	if err != nil {
		return nil, nil, errors.Wrapf(errors.ErrInner, "failed to get studio mindmap model, err=%v", err)
	}

	return chatModel, []einomodel.Option{
		llmchat.WithModel(model),
		llmchat.WithResponseJsonObject(provider),
	}, nil
}

func (m *mindmapGenerator) oneshotCreateMindmap(
	ctx context.Context,
	notebookId uuid.UUID,
	contents map[uuid.UUID]string,
	mode string,
) (*mindmapExpectation, error) {
	tmps := make([]string, 0, len(contents))
	for _, v := range contents {
		tmps = append(tmps, v)
	}

	var (
		msg  *einoschema.Message
		err  error
		lang = pkgcontext.GetLang(ctx)
	)

	if mode == mindmapAbstractMode {
		msg, err = prompts.RenderStudioMindmapAbstractMessage(ctx, tmps, lang)
	} else {
		msg, err = prompts.RenderStudioMindmapContentMessage(ctx, tmps, lang)
	}
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner,
			"failed to get mindmap template message, mode=%s, err=%v", mode, err)
	}

	chatModel, llmOptions, err := m.llmOptions()
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "failed to get studio mindmap model and options, err=%v", err)
	}
	msgs := pkgslices.FromSingle(msg)

	const retryTimes = 3
	for idx := range retryTimes {
		llmResp, err := chatModel.Generate(ctx, msgs, llmOptions...)
		if err != nil {
			return nil, errors.Wrapf(errors.ErrLLM,
				"failed to generate studio mindmap, retry=%d, err=%v",
				idx, err,
			)
		}

		var expect mindmapExpectation
		err = sonic.Unmarshal(pkgstring.AsBytes(llmResp.Content), &expect)
		if err != nil || !prompts.CheckStudioMindmapResult(expect.Mindmap) {
			// 给多一次机会
			var builder strings.Builder
			builder.WriteString("你刚才生成的思维导图不符合要求的格式，请重新输出，这个是你当前生成的结果:\n")
			builder.WriteString(llmResp.Content)
			builder.WriteString("\n\n")
			if err != nil {
				fmt.Fprintf(&builder, "原因：你输出的不是合法JSON，错误信息：%v\n\n请重新输出合法JSON", err)
			} else {
				builder.WriteString("原因：你输出的思维导图不符合要求的格式\n\n")
			}
			builder.WriteString("请严格按照格式要求重新输出，不要输出任何解释性文字")

			compensateMsg := &einoschema.Message{
				Role:    einoschema.User,
				Content: builder.String(),
			}
			msgs = append(msgs, compensateMsg)

			slog.WarnContext(ctx, "studio mindmap result not valid, compensating",
				slog.String("notebook_id", notebookId.String()))
			continue
		}

		return &expect, nil
	}

	return nil, errors.Wrap(errors.ErrLLM, "failed to generate studio mindmap")
}

func (m *mindmapGenerator) twoshotCreateMindmap(
	ctx context.Context,
	notebookId uuid.UUID,
	contents map[uuid.UUID]string,
	tokensCounts map[uuid.UUID]int,
) (*mindmapExpectation, error) {
	batches, err := m.splitContentBatches(ctx, contents, tokensCounts)
	if err != nil {
		return nil, errors.WithMessage(err, "split content batches failed")
	}

	// step1
	eg, ctx2 := errgroup.WithContext(ctx)
	resps := make([]string, len(batches))
	for idx, batch := range batches {
		eg.Go(safe.Do(ctx2, func() error {
			// 生成每个batch的思维导图
			mindmap, err := m.oneshotCreateMindmap(ctx,
				notebookId,
				map[uuid.UUID]string{uuid.NewV4(): batch.contents[0]},
				mindmapContentMode,
			)
			if err != nil {
				return errors.Wrapf(errors.ErrLLM, "generate mindmap failed, err=%v", err)
			}

			resps[idx] = mindmap.Mindmap
			return nil
		}))
	}
	err = eg.Wait()
	if err != nil {
		return nil, errors.WithMessage(err, "generate separate mindmap failed")
	}

	abstractContents := make(map[uuid.UUID]string, len(resps))
	for _, r := range resps {
		if r != "" {
			abstractContents[uuid.NewV4()] = r
		}
	}
	// step2
	expect, err := m.oneshotCreateMindmap(
		ctx,
		notebookId,
		abstractContents,
		mindmapAbstractMode,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "generate abstract mindmap failed")
	}

	return expect, nil
}

type batchContent struct {
	contents []string
}

func (m *mindmapGenerator) splitContentBatches(
	ctx context.Context,
	contents map[uuid.UUID]string,
	tokensCounts map[uuid.UUID]int,
) ([]*batchContent, error) {
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

	finalBatches := make([]*batchContent, 0, len(batches))
	for _, batch := range batches {
		if batch.totalTokens > constants.MindmapMaxOnceToken {
			// 检查每个batch是否超限 如果超限再拆开
			ccs := strings.Builder{}
			for _, c := range batch.contents {
				ccs.WriteString(c.content)
				ccs.WriteByte('\n')
			}
			splits, err := m.l.splitter.Transform(ctx,
				pkgslices.FromSingle(&einoschema.Document{
					Content: ccs.String(),
				}))
			if err != nil {
				return nil, errors.WithMessagef(err, "split content failed, id=%s", batch.contents[0].id)
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

	return finalBatches, nil
}
