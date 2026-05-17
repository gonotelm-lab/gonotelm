package source

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/mq"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infra/mq/impl"
	"github.com/gonotelm-lab/gonotelm/internal/infra/mq/impl/kafka"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	einoschema "github.com/cloudwego/eino/schema"
)

func mustNewMsgQueueProducer() mq.Producer {
	switch conf.Global().MsgQueue.Type {
	case mqimpl.Kafka:
		return kafka.NewProducer(kafka.ProducerConfig{
			Brokers:  conf.Global().MsgQueue.Kafka.Brokers,
			Username: conf.Global().MsgQueue.Kafka.Username,
			Password: conf.Global().MsgQueue.Kafka.Password,
		})
	default:
		panic("unknown msg queue type")
	}
}

func mustNewMsgQueueConsumer(topic, groupId string) mq.Consumer {
	switch conf.Global().MsgQueue.Type {
	case mqimpl.Kafka:
		return kafka.NewConsumer(kafka.ConsumerConfig{
			Brokers:        conf.Global().MsgQueue.Kafka.Brokers,
			GroupID:        groupId,
			Topic:          topic,
			QueueCapacity:  conf.Global().MsgQueue.Kafka.ConsumerQueueCapacity,
			CommitInterval: conf.Global().MsgQueue.Kafka.ConsumerCommitInterval,
			Username:       conf.Global().MsgQueue.Kafka.Username,
			Password:       conf.Global().MsgQueue.Kafka.Password,
		})
	default:
		panic("unknown msg queue type")
	}
}

// logic event handler
func (l *SourceLogic) notifySourceEventMessage(
	ctx context.Context,
	source *model.Source,
) error {
	sourceEvent := &sourceEventMessage{
		Id:         source.Id,
		NotebookId: source.NotebookId,
		Kind:       source.Kind,
		Status:     source.Status,
	}
	value, err := sonic.Marshal(sourceEvent)
	if err != nil {
		return errors.Wrapf(err,
			"marshal source failed, kind=%s, source_id=%s",
			source.Kind, source.Id)
	}

	err = l.prepProducer.Send(ctx, &mq.ProducerSendRequest{
		Topic:   TopicSourcePreparation,
		Key:     []byte(source.Id.String()),
		Value:   value,
		Headers: nil, // TODO add trace headers?
	})
	if err != nil {
		return errors.Wrapf(err,
			"send source inserted message failed, kind=%s, source_id=%s",
			source.Kind, source.Id)
	}

	return nil
}

// 消息队列消费 消费上传完成的任务 指定来源索引构建
func (l *SourceLogic) handleSourceEventMessage(
	ctx context.Context,
	msg mq.Message,
) error {
	var (
		key = msg.Key()
		val = msg.Value()

		sourceEvent sourceEventMessage
		err         error
	)

	sourceId, _ := uuid.FromBytes(key)
	slog.DebugContext(ctx, "received and handling source prep message",
		slog.String("msg_key", sourceId.String()),
	)
	err = sonic.Unmarshal(val, &sourceEvent)
	if err != nil {
		return errors.Wrap(err, "handle prep message unmarshal failed")
	}

	result, err := l.sourceBiz.PrepareSourceIndices(ctx, sourceEvent.Id)
	if err != nil {
		// mark failure
		err2 := l.sourceBiz.UpdateStatus(ctx, sourceEvent.Id, model.SourceStatusFailed)
		if err2 != nil {
			return errors.WithMessage(err2, "update source status failed")
		}

		if errors.Is(err, bizsource.ErrSourceContentTooLong) {
			slog.WarnContext(ctx, "source content too long",
				slog.String("source_id", sourceEvent.Id.String()),
				slog.Any("err", err),
			)
			return nil
		}

		slog.ErrorContext(ctx, "prepare source failed",
			slog.String("source_id", sourceEvent.Id.String()),
			slog.Any("err", err),
		)
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(2)
	l.wg.Go(func() {
		l.generateParsedContent(ctx, sourceEvent.Id, sourceEvent.NotebookId, result)
		wg.Done()
	})
	l.wg.Go(func() {
		l.generateSourceSummary(ctx, sourceEvent.Id, sourceEvent.NotebookId, result)
		wg.Done()
	})
	wg.Wait()

	l.generateNotebookSummary(ctx, sourceEvent.NotebookId)

	// ok
	err = l.sourceBiz.UpdateStatus(ctx, sourceEvent.Id, model.SourceStatusReady)
	if err != nil {
		return errors.WithMessage(err, "update source status failed")
	}

	slog.DebugContext(ctx, "prepared source success", "source_id", sourceEvent.Id)

	return nil
}

func (l *SourceLogic) generateParsedContent(
	ctx context.Context,
	sourceId uuid.UUID,
	notebookId uuid.UUID,
	result *bizsource.PrepareSourceIndicesResult,
) {
	if result.ParsedContent == nil {
		return
	}

	storeKey := formatSourceParsedContentStoreKey(sourceId, notebookId)
	err := l.objectStorage.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         storeKey,
		Body:        result.ParsedContent,
		ContentType: result.ParsedContentType,
	})
	// 解析成功 但是上传失败 仅打日志不影响后续流程
	if err != nil {
		slog.ErrorContext(ctx, "upload parsed content failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)

		return
	}

	err = l.sourceBiz.UpdateParsedContent(ctx, &bizsource.UpdateParsedContentCommand{
		Id: sourceId,
		Parsed: &model.ParsedSourceContent{
			StoreKey: storeKey,
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, "update source parsed content failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)
	}
}

func (l *SourceLogic) generateSourceSummary(
	ctx context.Context,
	sourceId uuid.UUID,
	notebookId uuid.UUID,
	result *bizsource.PrepareSourceIndicesResult,
) {
	if len(result.Chunks) == 0 {
		return
	}

	summaryModel, err := l.llmGateway.GetProvider(
		conf.Global().Logic.Source.ModelProvider,
	)
	if err != nil {
		slog.ErrorContext(ctx, "get summary model failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)
		return
	}
	var (
		llmOption = llmchat.BuildLLMModelOption(conf.Global().Logic.Source.Model)
		userLang  = "" // TODO
	)

	const (
		batchSize          = 1
		maxConcurrency     = 20
		tokenSize          = 10000
		maxSummarizedChunk = 25
	)

	newChunks := pkgstring.MergeChunks(result.Chunks, tokenSize)
	if len(newChunks) > maxSummarizedChunk {
		newChunks = newChunks[:maxSummarizedChunk]
	}

	// newChunks中每个元素都生成一份摘要
	chunkSummaries, err := batch.ParallelMap(
		ctx,
		newChunks,
		batchSize,
		maxConcurrency,
		func(ctx context.Context, batch []string) ([]string, error) {
			summaryPrompt, err := prompts.SummarizePromptMessage(
				ctx, batch[0], userLang,
			)
			if err != nil {
				slog.ErrorContext(ctx, "render summarize prompt failed",
					slog.String("source_id", sourceId.String()),
					slog.Any("err", err),
				)
				return []string{}, nil
			}

			msgs := []*einoschema.Message{summaryPrompt}
			result, err := summaryModel.Generate(ctx, msgs, llmOption)
			if err != nil {
				slog.ErrorContext(ctx, "generate summary failed",
					slog.String("source_id", sourceId.String()),
					slog.Any("err", err),
				)
				return []string{}, nil
			}

			return []string{result.Content}, nil
		},
	)
	if err != nil {
		slog.ErrorContext(ctx, "generate summary failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)
		return
	}

	// 给那个chunk的summary再输出一句summary
	summarizingTexts := strings.Join(chunkSummaries, "\n")
	summaryPrompt, err := prompts.SummarizePromptMessage(
		ctx, summarizingTexts, userLang,
	)
	if err != nil {
		slog.ErrorContext(ctx, "render summarize prompt failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)
		return
	}

	msgs := []*einoschema.Message{summaryPrompt}
	summaryResult, err := summaryModel.Generate(ctx, msgs, llmOption)
	if err != nil {
		slog.ErrorContext(ctx, "generate summary failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)
		return
	}

	if err := l.sourceBiz.UpdateAbstract(ctx, sourceId, summaryResult.Content); err != nil {
		slog.ErrorContext(ctx, "update source abstract failed",
			slog.String("source_id", sourceId.String()),
			slog.Any("err", err),
		)
	}
}

func (l *SourceLogic) generateNotebookSummary(
	ctx context.Context,
	notebookId uuid.UUID,
) {
	slog.DebugContext(ctx, "generate notebook summary",
		slog.String("notebook_id", notebookId.String()),
	)

	notebook, err := l.notebookBiz.GetNotebook(ctx, notebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return
		}

		slog.ErrorContext(ctx, "get notebook failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	if notebook.Description != "" {
		// 自动生成的描述不覆盖已有的
		return
	}

	// get all notebook sources
	notebookSources, err := l.sourceBiz.GetAllNotebookSources(ctx, notebookId)
	if err != nil {
		slog.ErrorContext(ctx, "get all notebook sources failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	abstracts := make([]string, 0, len(notebookSources))
	for _, source := range notebookSources {
		if source.Abstract != "" {
			abstracts = append(abstracts, source.Abstract)
		}
	}

	userLang := "" // TODO
	// generate prompt message
	msg, err := prompts.NotebookSummaryPromptMessage(
		ctx, abstracts, userLang,
	)
	if err != nil {
		slog.ErrorContext(ctx, "render notebook summary prompt failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	// generate summary
	provider, err := l.llmGateway.GetProvider(
		conf.Global().Logic.Source.ModelProvider,
	)
	if err != nil {
		slog.ErrorContext(ctx, "get summary model failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}
	llmOption := llmchat.BuildLLMModelOption(conf.Global().Logic.Source.Model)
	result, err := provider.Generate(
		ctx,
		[]*einoschema.Message{msg},
		llmOption,
	)
	if err != nil {
		slog.ErrorContext(ctx, "generate notebook summary failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	// now we update notebook description
	convention := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}{}

	// truncate
	convention.Name = strings.TrimSpace(convention.Name)
	convention.Description = strings.TrimSpace(convention.Description)
	convention.Name = constants.TruncateNotebookName(convention.Name)
	convention.Description = constants.TruncateNotebookDescription(convention.Description)

	err = sonic.Unmarshal(pkgstring.AsBytes(result.Content), &convention)
	if err != nil {
		slog.WarnContext(ctx, "llm model response unmarshal failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}

	slog.DebugContext(ctx, "update notebook description",
		slog.String("notebook_id", notebookId.String()),
	)

	err = l.notebookBiz.FillNotebookMeta(ctx,
		&biznotebook.FillNotebookMetaCommand{
			Id:          notebookId,
			Name:        convention.Name,
			Description: convention.Description,
		})
	if err != nil {
		slog.ErrorContext(ctx, "fill notebook meta failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
		return
	}
}
