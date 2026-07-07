package studio

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
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

	mindmapTitleMinLen = 10
	mindmapTitleMaxLen = 30
)

type mindmapGenerator struct {
	l *Logic
}

var (
	_ taskHandler       = &mindmapGenerator{}
	_ iCommonTaskParams = &generateMindmapTaskParams{}
)

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

	slog.DebugContext(ctx, "perform agent create mindmap", slog.Int("total_tokens", totalTokens))

	// return m.twoshotCreateMindmap(ctx, params.NotebookId, parsedContents, tokensCounts)
	return m.agentCreateMindmap(ctx, params, sources)
}

func (m *mindmapGenerator) llmOptions() []einomodel.Option {
	var (
		provider = conf.Global().Logic.Studio.Mindmap.ModelProvider
		model    = conf.Global().Logic.Studio.Mindmap.Model
	)
	return []einomodel.Option{
		llm.WithModel(model),
		llm.WithResponseJsonObject(provider),
		llm.WithThinking(provider, false),
	}
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
		provider = conf.Global().Logic.Studio.Mindmap.ModelProvider
		msgs     []*einoschema.Message
		err      error
		lang     = pkgcontext.GetLang(ctx)
	)

	if mode == mindmapAbstractMode {
		msgs, err = m.l.prompt.RenderStudioMindmapAbstractMessage(ctx, tmps, lang)
	} else {
		msgs, err = m.l.prompt.RenderStudioMindmapContentMessage(ctx, tmps, lang)
	}
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner,
			"failed to get mindmap template message, mode=%s, err=%v", mode, err)
	}

	llmOptions := m.llmOptions()
	msgs = append([]*einoschema.Message{}, msgs...)
	const retryTimes = 3
	chatModel, err := m.l.llmGateway.GetProvider(provider)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get llm gateway provider %s", provider)
	}
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
		if err != nil || !bizprompt.CheckStudioMindmapResult(expect.Mindmap) {
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

// Deprecated: 这个太消耗token了 而且很慢
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

// 在大文本下 让agent先使用工具探索
// 可以使用stat, grep, query等工具先探索主题相关内容 可大幅节省token
// 然后再让agent生成思维导图
func (m *mindmapGenerator) agentCreateMindmap(
	ctx context.Context,
	params *generateMindmapTaskParams,
	sources []*model.DecodedSource,
) (*mindmapExpectation, error) {
	llmOptions := m.llmOptions()

	ag, err := m.l.buildSourceExploreAgent(
		conf.Global().Logic.Studio.Mindmap.ModelProvider,
		conf.Global().Logic.Studio.Mindmap.Model,
		conf.Global().Logic.Studio.Mindmap.MaxRound,
		llmOptions,
		params,
		true,
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to build source explore agent for mindmap")
	}

	sourceIds := sourceIDsToStrings(decodedSourcesToSourceIDs(sources))
	msgs, err := m.l.prompt.RenderStudioMindmapV2Message(ctx, sourceIds, pkgcontext.GetLang(ctx))
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate mindmap message failed, err=%v", err)
	}
	output, err := ag.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate mindmap output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate mindmap agent usage: %+v", ag.TokenUsage()))

	expect, err := m.parseAgentOutput(ctx, output.Content)
	if err == nil {
		return expect, nil
	}

	slog.WarnContext(ctx, "mindmap agent output invalid, compensating",
		slog.String("notebook_id", params.getNotebookId().String()),
		slog.String("output", string(output.Content)),
		slog.Any("usage", ag.TokenUsage()),
	)

	// agent 最终输出可能夹带思考文本；这里做一次无工具纠偏，强制只输出目标 JSON。
	msgs = append([]*einoschema.Message{}, ag.AccumulatedMessages()...)
	compensateMsg := &einoschema.Message{
		Role: einoschema.User,
		Content: fmt.Sprintf(
			"你刚才输出的结果不符合要求，请严格重输。\n当前输出：\n%s\n\n"+
				"要求：\n"+
				"1) 只输出一个合法 JSON 对象，不要任何解释性文字\n"+
				"2) JSON 字段必须且仅能包含 title 和 mindmap\n"+
				"3) title 长度必须为 10-30 字\n"+
				"4) mindmap 必须是完整 mermaid mindmap 代码块字符串\n"+
				"5) 不允许输出 ```json 代码块包裹",
			output.Content,
		),
	}
	msgs = append(msgs, compensateMsg)

	llmResp, genErr := ag.BaseLLM().Generate(ctx, msgs, llmOptions...)
	if genErr != nil {
		return nil, errors.Wrapf(errors.ErrLLM,
			"mindmap compensate generate failed, err=%v",
			genErr,
		)
	}

	expect, err = m.parseAgentOutput(ctx, llmResp.Content)
	if err == nil {
		return expect, nil
	}

	return nil, errors.Wrapf(errors.ErrLLM,
		"mindmap agent output invalid after compensation, first_output=%q, compensate_output=%q, err=%v",
		output.Content,
		llmResp.Content,
		err,
	)
}

func (m *mindmapGenerator) parseAgentOutput(ctx context.Context, content string) (*mindmapExpectation, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect mindmapExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.WarnContext(ctx, "mindmap direct output unmarshal failed, fallback to extracted json candidates",
				slog.Any("err", err))
		},
	}
	// Decoder.Unmarshal 会先直接解析，失败时自动提取 JSON 片段再解析。
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "mindmap output unmarshal failed after compatibility fallback",
			slog.Any("err", err))
		return nil, err
	}

	expect.Title = strings.TrimSpace(expect.Title)
	expect.Mindmap = strings.TrimSpace(expect.Mindmap)

	titleLen := utf8.RuneCountInString(expect.Title)
	if titleLen > mindmapTitleMinLen {
		// truncate title
		expect.Title = pkgstring.TruncateRune(expect.Title, mindmapTitleMaxLen)
	}

	if !bizprompt.CheckStudioMindmapResult(expect.Mindmap) {
		return nil, fmt.Errorf("mindmap format invalid")
	}

	return &expect, nil
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
