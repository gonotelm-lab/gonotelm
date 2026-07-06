package database

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Id = uuid.UUID

type NotebookStore interface {
	Create(ctx context.Context, notebook *schema.Notebook) error
	Upsert(ctx context.Context, notebook *schema.Notebook) error
	GetById(ctx context.Context, id Id) (*schema.Notebook, error)
	GetByNameAndOwnerId(ctx context.Context, name, ownerId string) (*schema.Notebook, error)
	// orderBy=0 -> default order; orderBy=1 -> order by updated_at
	ListByOwnerId(ctx context.Context, ownerId string, limit, offset, orderBy int) ([]*schema.Notebook, error)
	Update(ctx context.Context, notebook *schema.Notebook) error
	DeleteById(ctx context.Context, id Id) error
	UpdateName(ctx context.Context, params *schema.NotebookUpdateNameParams) error
	UpdateDescription(ctx context.Context, params *schema.NotebookUpdateDescriptionParams) error
	// 仅填充空的name/description，非空字段保持不变
	FillNameAndDescriptionIfEmpty(ctx context.Context, params *schema.NotebookFillNameAndDescriptionParams) error
}

type SourceStore interface {
	Create(ctx context.Context, source *schema.Source) error
	Upsert(ctx context.Context, source *schema.Source) error
	GetById(ctx context.Context, id Id) (*schema.Source, error)
	CountByNotebookId(ctx context.Context, notebookId Id) (int64, error)
	BatchCountByNotebookIds(ctx context.Context, notebookIds []Id) (map[Id]int64, error)
	ListByNotebookId(ctx context.Context, notebookId Id, limit, offset int) ([]*schema.Source, error)
	DeleteById(ctx context.Context, id Id) error
	BatchDelete(ctx context.Context, ids []Id) error
	DeleteByNotebookId(ctx context.Context, notebookId Id) error
	UpdateStatus(ctx context.Context, params *schema.SourceUpdateStatusParams) error
	Update(ctx context.Context, params *schema.SourceUpdateParams) error
	UpdateTitle(ctx context.Context, params *schema.SourceUpdateTitleParams) error
	ListByIds(ctx context.Context, ids []Id) ([]*schema.Source, error)
	ListByNotebookIdAndIds(ctx context.Context, notebookId Id, ids []Id) ([]*schema.Source, error)
	UpdateParsedContent(ctx context.Context, params *schema.SourceUpdateParsedContentParams) error
	UpdateAbstract(ctx context.Context, params *schema.SourceUpdateAbstractParams) error
}

type ChatStore interface {
	Create(ctx context.Context, chat *schema.Chat) error
	GetById(ctx context.Context, id Id) (*schema.Chat, error)
	GetByNotebookIdAndOwnerId(ctx context.Context, notebookId Id, ownerId string) (*schema.Chat, error)
	ListByNotebookId(ctx context.Context, notebookId Id) ([]*schema.Chat, error)
	ListByOwnerId(ctx context.Context, ownerId string, limit, offset int) ([]*schema.Chat, error)
	DeleteById(ctx context.Context, id Id) error
	DeleteByNotebookId(ctx context.Context, notebookId Id) error
}

type ChatMessageStore interface {
	Create(ctx context.Context, message *schema.ChatMessage) error
	GetById(ctx context.Context, id Id) (*schema.ChatMessage, error)
	GetByIdAndChatId(ctx context.Context, id Id, chatId Id) (*schema.ChatMessage, error)
	// 按照seqno从大到小排序
	ListByChatId(ctx context.Context, chatId Id, limit, offset int) ([]*schema.ChatMessage, error)
	// 按照seqno从大到小排序, 查询seq_no < beforeSeqNo的消息
	ListByChatIdBeforeSeqNo(ctx context.Context, chatId Id, beforeSeqNo int64, limit int) ([]*schema.ChatMessage, error)
	DeleteByChatId(ctx context.Context, chatId Id) error
	BatchDeleteByChatIds(ctx context.Context, chatIds []Id) error
}

type ArtifactTaskStore interface {
	// 创建任务
	Create(ctx context.Context, task *schema.ArtifactTask) error

	// 根据id获取任务
	GetById(ctx context.Context, id Id) (*schema.ArtifactTask, error)

	// 根据notebookId和id获取任务
	GetByNotebookIdAndId(ctx context.Context, notebookId, id Id) (*schema.ArtifactTask, error)

	// 获取任务状态
	GetStatusById(ctx context.Context, id Id) (string, error)

	// 按照created_at DESC分页获取
	ListByNotebookId(ctx context.Context, notebookId Id, limit, offset int) ([]*schema.ArtifactTask, error)

	DeleteByNotebookId(ctx context.Context, notebookId Id) error

	// 按照NotebookId分页获取
	PageListByNotebookId(
		ctx context.Context,
		notebookId Id,
		cursor Id, limit int,
	) ([]*schema.ArtifactTask, error)

	// 认领任务
	// 返回值：task, true, nil => 成功认领
	// 返回值：nil, false, nil => 没有任务可认领，且没有错误
	// 返回值：nil, false, err => 出错
	Claim(ctx context.Context,
		oldStatus string,
		now int64,
		params *schema.ArtifactTaskClaimParams,
	) (*schema.ArtifactTask, bool, error)

	// 强行设置任务状态
	//
	// 当前任务状态为oldStatus时设置新状态
	SetStatus(ctx context.Context,
		id Id,
		newStatus string,
		oldStatuses []string,
		updatedAt int64,
		expiredAt int64, // -1 means no change
	) error

	// 批量设置任务状态
	// 当前任务状态为oldStatus时设置新状态
	BatchSetStatus(ctx context.Context,
		ids []Id,
		newStatus string,
		oldStatuses []string,
		updatedAt int64,
		expiredAt int64, // -1 means no change
	) error

	// 更新任务状态
	UpdateStatus(
		ctx context.Context,
		id Id,
		runId string,
		oldStatus string,
		params *schema.ArtifactTaskUpdateStatusParams,
	) (bool, error)

	// 更新任务结果
	UpdateResult(
		ctx context.Context,
		id Id,
		runId string,
		oldStatus string,
		params *schema.ArtifactTaskUpdateResultParams,
	) (bool, error)

	// 删除任务
	DeleteById(ctx context.Context, id Id) error

	// 根据id和状态删除任务
	DeleteByIdAndNotStatus(ctx context.Context, id Id, status string) (bool, error)

	// 更新过期的任务状态
	SetExpiredTasksStatus(
		ctx context.Context,
		ids []Id,
		newStatus string,
		updatedAt int64,
		now int64,
	) error

	// 列出过期的任务
	PageListExpiredTasks(
		ctx context.Context,
		cursor Id, // id > cursor
		limit int,
		now int64,
	) ([]*schema.ArtifactTask, error)
}

type DAL struct {
	Closer misc.Closer

	NotebookStore     NotebookStore
	SourceStore       SourceStore
	ChatStore         ChatStore
	ChatMessageStore  ChatMessageStore
	ArtifactTaskStore ArtifactTaskStore
}

func NewDAL(
	closer misc.Closer,
	notebookStore NotebookStore,
	sourceStore SourceStore,
	chatStore ChatStore,
	chatMessageStore ChatMessageStore,
	artifactTaskStore ArtifactTaskStore,
) *DAL {
	return &DAL{
		Closer:            closer,
		NotebookStore:     notebookStore,
		SourceStore:       sourceStore,
		ChatStore:         chatStore,
		ChatMessageStore:  chatMessageStore,
		ArtifactTaskStore: artifactTaskStore,
	}
}

func (d *DAL) Close(ctx context.Context) error {
	slog.WarnContext(ctx, "closing database connections...")
	return d.Closer.Close(ctx)
}

