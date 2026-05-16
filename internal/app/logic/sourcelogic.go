package logic

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"

	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/mq"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
)

// mq topic names
const (
	TopicSourcePreparation = "gonotelm.source.preparation"
)

// consumer group id
const (
	SourcePreparationConsumerGroup = "gonotelm.source.preparation.group"
	maxUploadFileSizeBytes         = 100 * 1024 * 1024
)

type sourceEventMessage struct {
	Id         uuid.UUID          `json:"id"`
	NotebookId uuid.UUID          `json:"notebook_id"`
	Kind       model.SourceKind   `json:"kind"`
	Status     model.SourceStatus `json:"status"`
}

type SourceLogic struct {
	rootCtx       context.Context
	objectStorage storage.Storage

	notebookBiz *biznotebook.Biz

	sourceBiz    *bizsource.Biz
	prepProducer mq.Producer
	prepConsumer mq.Consumer
}

func (l *SourceLogic) Close(ctx context.Context) {
	if l.prepConsumer != nil {
		if err := l.prepConsumer.Close(ctx); err != nil {
			slog.WarnContext(ctx, "close source prep consumer failed", slog.Any("err", err))
		}
	}
	if l.prepProducer != nil {
		if err := l.prepProducer.Close(ctx); err != nil {
			slog.WarnContext(ctx, "close source prep producer failed", slog.Any("err", err))
		}
	}
}

func MustNewSourceLogic(
	rootCtx context.Context,
	objectStorage storage.Storage,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
) *SourceLogic {
	sl := &SourceLogic{
		rootCtx:       rootCtx,
		objectStorage: objectStorage,
		notebookBiz:   notebookBiz,
		sourceBiz:     sourceBiz,
	}

	sl.mustInitMsgQueue()

	return sl
}

func (l *SourceLogic) mustInitMsgQueue() {
	// producer
	l.prepProducer = mustNewMsgQueueProducer()
	// consumer
	l.prepConsumer = mustNewMsgQueueConsumer(TopicSourcePreparation, SourcePreparationConsumerGroup)
	l.prepConsumer.Subscribe(l.rootCtx, TopicSourcePreparation, l.handleSourceEventMessage)
}

type CreateSourceParams struct {
	NotebookId uuid.UUID
	Kind       model.SourceKind // text, url, file
	Text       string           // text kind
	Url        *url.URL         // url kind
}

func (l *SourceLogic) CreateSource(
	ctx context.Context,
	params *CreateSourceParams,
) (*model.Source, error) {
	userId := pkgcontext.GetUserId(ctx)
	// check if notebook exists
	_, err := l.notebookBiz.GetNotebook(ctx, params.NotebookId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, errors.ErrParams.Msgf("notebook not found, id=%s", params.NotebookId)
		}

		return nil, errors.WithMessagef(err, "get notebook failed, id=%s", params.NotebookId)
	}

	source, err := l.sourceBiz.CreateSource(
		ctx, &bizsource.CreateSourceCommand{
			NotebookId:  params.NotebookId,
			OwnerId:     userId,
			Kind:        params.Kind,
			TextContent: params.Text,
			UrlContent:  params.Url,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "create source failed")
	}

	if !source.KindFile() {
		// not file kind, we can do preparing immediately
		sourceValue, err := sonic.Marshal(source)
		if err != nil {
			return nil, errors.Wrap(err, "marshal source failed")
		}
		err = l.prepProducer.Send(ctx, &mq.ProducerSendRequest{
			Topic:   TopicSourcePreparation,
			Key:     source.Id.Bytes(),
			Value:   sourceValue,
			Headers: nil,
		})
		if err != nil {
			return nil, errors.Wrap(err, "send source inserted message failed")
		}
	}

	return source, nil
}

func (l *SourceLogic) GetSource(
	ctx context.Context,
	id uuid.UUID,
) (*model.Source, error) {
	source, err := l.sourceBiz.GetSource(ctx, id)
	if err != nil {
		return nil, errors.WithMessage(err, "get source failed")
	}

	return source, nil
}

type GetSourceDocParams struct {
	SourceId uuid.UUID
	DocId    string
}

type GetSourceDocResult struct {
	SourceId    string
	DocId       string
	SourceTitle string
	Content     string
}

func (l *SourceLogic) GetSourceDoc(
	ctx context.Context,
	params *GetSourceDocParams,
) (*GetSourceDocResult, error) {
	userId := pkgcontext.GetUserId(ctx)

	source, err := l.GetSource(ctx, params.SourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source failed, source_id=%s", params.SourceId)
	}
	if source.OwnerId != userId {
		return nil, errors.ErrPermission.Msg("source access denied")
	}

	doc, err := l.sourceBiz.GetSourceDoc(ctx, &bizsource.GetSourceDocQuery{
		NotebookId: source.NotebookId,
		SourceId:   source.Id,
		DocId:      params.DocId,
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "get source doc failed, source_id=%s, doc_id=%s", source.Id, params.DocId)
	}

	return &GetSourceDocResult{
		SourceId:    source.Id.String(),
		DocId:       doc.Id,
		SourceTitle: source.DisplayName,
		Content:     doc.Content,
	}, nil
}

type UploadSourceParams struct {
	SourceId uuid.UUID
	Filename string
	MimeType string
	Size     int64
	Md5      string
}

type UploadSourceResult struct {
	Url     string            `json:"url"`
	Method  string            `json:"method"`
	Forms   map[string]string `json:"forms"`
	Headers map[string]string `json:"headers"`
}

func (l *SourceLogic) UploadFileSource(
	ctx context.Context,
	params *UploadSourceParams,
) (*UploadSourceResult, error) {
	if params.Size > maxUploadFileSizeBytes {
		return nil, errors.ErrParams.Msgf("file size must be less than or equal to %d bytes", maxUploadFileSizeBytes)
	}
	if !model.SupportedFileMimeType(params.MimeType) {
		return nil, errors.ErrParams.Msgf("unsupported mime_type: %s", params.MimeType)
	}

	source, err := l.GetSource(ctx, params.SourceId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source failed, source_id=%s", params.SourceId)
	}

	if !checkSourceUploadable(source) {
		return nil, errors.ErrParams.Msgf("source is not uploadable, kind=%s, status=%s", source.Kind, source.Status)
	}

	storeKey := formatSourceStoreKey(params, source)
	policy, err := l.objectStorage.PresignedPostPolicy(ctx,
		&storage.PresignedPostPolicyRequest{
			Key:           storeKey,
			ContentType:   params.MimeType,
			ContentLength: params.Size,
			Filename:      params.Filename,
			Md5:           params.Md5,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "presigned post policy failed")
	}

	content := model.FileSourceContent{
		StoreKey: storeKey,
		Filename: params.Filename,
		Size:     params.Size,
		Md5:      params.Md5,
		Format:   params.MimeType,
	}
	// store file data as source content
	err = l.updateFileSourceContent(ctx, source, &content)
	if err != nil {
		return nil, errors.WithMessage(err, "store update source failed")
	}

	return &UploadSourceResult{
		Url:     policy.Url,
		Method:  policy.Method,
		Forms:   policy.Forms,
		Headers: policy.Headers,
	}, nil
}

// check source status and update if necessary
func (l *SourceLogic) PollSourceStatus(
	ctx context.Context,
	sourceId uuid.UUID,
) (model.SourceStatus, error) {
	target, err := l.GetSource(ctx, sourceId)
	if err != nil {
		// source may be deleted while frontend polling status; treat as terminal status.
		if errors.Is(err, bizsource.ErrSourceNotFound) || errors.Is(err, errors.ErrNoRecord) {
			return model.SourceStatusFailed, nil
		}
		return "", errors.WithMessagef(err, "get source failed, id=%s", sourceId)
	}

	if target.StatusReady() {
		return target.Status, nil
	}

	var status model.SourceStatus = target.Status
	if target.KindFile() {
		status, err = l.pollFileSourceStatus(ctx, target)
		// lazy update source status
		if err != nil {
			return "", errors.WithMessage(err, "poll source status failed")
		}
	}

	return status, nil
}

// 失败时可以重新构建来源
func (l *SourceLogic) RetrySourcePreparation(
	ctx context.Context,
	sourceId uuid.UUID,
) error {
	source, err := l.GetSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, bizsource.ErrSourceNotFound) {
			return nil
		}

		return errors.WithMessagef(err, "get source failed, id=%s", sourceId)
	}

	if !source.StatusFailed() {
		return errors.ErrParams.Msg("no need to retry")
	}

	err = l.sourceBiz.UpdateStatus(ctx, source.Id, model.SourceStatusPreparing)
	if err != nil {
		return errors.WithMessage(err, "update source status to preparing failed")
	}
	source.Status = model.SourceStatusPreparing

	// do retry by re-sending the source event message
	err = l.notifySourceEventMessage(ctx, source)
	if err != nil {
		rollbackErr := l.sourceBiz.UpdateStatus(ctx, source.Id, model.SourceStatusFailed)
		if rollbackErr != nil {
			slog.WarnContext(ctx, "rollback source status after retry notify failure failed",
				slog.String("source_id", source.Id.String()),
				slog.Any("err", rollbackErr))
		}
		return errors.WithMessage(err, "notify source preparing failed")
	}

	return nil
}

func (l *SourceLogic) DeleteSource(ctx context.Context, sourceId uuid.UUID) error {
	err := l.sourceBiz.DeleteSource(ctx, sourceId)
	if err != nil {
		return errors.WithMessagef(err, "delete source failed, id=%s", sourceId)
	}

	return nil
}

type GetSourceParsedContentResult struct {
	Content string `json:"content,omitempty"`
	Url     string `json:"url,omitempty"`
}

func (l *SourceLogic) GetSourceParsedContent(
	ctx context.Context,
	sourceId uuid.UUID,
) (*GetSourceParsedContentResult, error) {
	source, err := l.sourceBiz.GetDecodedSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, bizsource.ErrSourceNotFound) {
			return nil, errors.ErrParams.Msgf("source not found, id=%s", sourceId)
		}

		return nil, errors.WithMessagef(err, "get source failed, id=%s", sourceId)
	}

	userId := pkgcontext.GetUserId(ctx)
	// check user permission
	if source.OwnerId != userId {
		return nil, errors.ErrPermission.Msg("source access denied")
	}

	if source.Status != model.SourceStatusReady {
		return nil, errors.ErrParams.Msgf("source is not ready, status=%s", source.Status)
	}

	if source.Kind == model.SourceKindText {
		return &GetSourceParsedContentResult{
			Content: source.ContentText.Text,
		}, nil
	}

	var storeKey string
	if source.ParsedContent != nil {
		storeKey = source.ParsedContent.StoreKey
	}

	if storeKey == "" {
		// slog.WarnContext(ctx, "parsed content store key is empty", "source_id", sourceId)
		return &GetSourceParsedContentResult{}, nil
	}

	resp, err := l.objectStorage.PresignedGetObject(ctx,
		&storage.PresignedGetObjectRequest{
			Key: storeKey,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "get presigned get object failed")
	}

	return &GetSourceParsedContentResult{
		Url: resp.Url,
	}, nil
}

func (l *SourceLogic) pollFileSourceStatus(
	ctx context.Context,
	source *model.Source,
) (status model.SourceStatus, err error) {
	// check if file already uploaded
	fileSource := model.FileSourceContent{}
	status = source.Status

	if len(source.Content) != 0 { // normally empty content means inited status
		err = sonic.Unmarshal(source.Content, &fileSource)
		if err != nil {
			err = errors.Wrapf(errors.ErrSerde,
				"unmarshal file source failed, kind=%s, source_id=%s, err=%v",
				source.Kind, source.Id, err)
			return
		}
	}

	if !source.StatusUploading() {
		return source.Status, nil
	}

	// uploading status, check if file already uploaded
	_, err = l.objectStorage.StatObject(ctx, &storage.StatObjectRequest{
		Key: fileSource.StoreKey,
	})
	// TODO check if object is actually supported from mime type
	if err != nil {
		if !errors.Is(err, storage.ErrObjectNotFound) {
			err = errors.WithMessagef(err, "stat object failed, key=%s", fileSource.StoreKey)
			return
		}
		err = nil
	} else {
		// uploaded, make it preparing
		err = l.notifySourceEventMessage(ctx, source)
		if err != nil {
			err = errors.WithMessage(err, "notify source preparing failed")
			return
		}

		// update source status
		err = l.sourceBiz.UpdateStatus(ctx, source.Id, model.SourceStatusPreparing)
		if err != nil {
			err = errors.WithMessage(err, "update source status failed")
			return
		}
		status = model.SourceStatusPreparing
	}

	return
}

func (l *SourceLogic) updateFileSourceContent(
	ctx context.Context,
	source *model.Source,
	fileContent *model.FileSourceContent,
) error {
	if source == nil {
		return errors.ErrParams.Msg("source is nil")
	}
	if fileContent == nil {
		return errors.ErrParams.Msg("file source content is nil")
	}

	content, err := sonic.Marshal(fileContent)
	if err != nil {
		return errors.WithMessage(err, "marshal file source failed")
	}

	return l.sourceBiz.UpdateContent(ctx, &bizsource.UpdateContentCommand{
		Id:          source.Id,
		Status:      model.SourceStatusUploading,
		DisplayName: fileContent.Filename,
		Content:     content,
	})
}

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
		key    = msg.Key()
		val    = msg.Value()
		source sourceEventMessage
		err    error
	)

	sourceId, _ := uuid.FromBytes(key)
	slog.DebugContext(ctx, "received and handling source prep message", "msg_key", sourceId.String())
	err = sonic.Unmarshal(val, &source)
	if err != nil {
		return errors.Wrap(err, "handle prep message unmarshal failed")
	}

	result, err := l.sourceBiz.PrepareSourceIndices(ctx, source.Id)
	if err != nil {
		// mark failure
		err2 := l.sourceBiz.UpdateStatus(ctx, source.Id, model.SourceStatusFailed)
		if err2 != nil {
			return errors.WithMessage(err2, "update source status failed")
		}

		slog.ErrorContext(ctx, "prepare source failed", "source_id", source.Id, "err", err)
		return nil
	}

	if result.ParsedContent != nil {
		storeKey := formatSourceParsedContentStoreKey(source.Id, source.NotebookId)
		err = l.objectStorage.UploadObject(ctx, &storage.UploadObjectRequest{
			Key:         storeKey,
			Body:        result.ParsedContent,
			ContentType: result.ParsedContentType,
		})
		// 解析成功 但是上传失败 仅打日志不影响后续流程
		if err != nil {
			slog.ErrorContext(ctx, "upload parsed content failed", "source_id", source.Id, "err", err)
		} else {
			err = l.sourceBiz.UpdateParsedContent(ctx, &bizsource.UpdateParsedContentCommand{
				Id: source.Id,
				Parsed: &model.ParsedSourceContent{
					StoreKey: storeKey,
				},
			})
			if err != nil {
				slog.ErrorContext(ctx, "update source parsed content failed", "source_id", source.Id, "err", err)
			}
		}
	}

	// ok
	err = l.sourceBiz.UpdateStatus(ctx, source.Id, model.SourceStatusReady)
	if err != nil {
		return errors.WithMessage(err, "update source status failed")
	}

	slog.DebugContext(ctx, "prepared source success", "source_id", source.Id)

	return nil
}

func checkSourceUploadable(source *model.Source) bool {
	return source.KindFile() && source.StatusInited()
}

// Format:
// file/{{notebook_id}}/{{source_id}}{{.format}}
func formatSourceStoreKey(
	params *UploadSourceParams,
	source *model.Source,
) string {
	var (
		notebookId = source.NotebookId.String()
		sourceId   = source.Id.String()
		// take extension from input filename
		ext = filepath.Ext(params.Filename)
	)

	return fmt.Sprintf("file/%s/%s%s", notebookId, sourceId, ext)
}

// Format:
// parsed_file/{{notebook_id}}/{{source_id}}
func formatSourceParsedContentStoreKey(
	sourceId, notebookId uuid.UUID,
) string {
	return fmt.Sprintf("parsed_file/%s/%s", notebookId.String(), sourceId.String())
}
