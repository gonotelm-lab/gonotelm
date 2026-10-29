package database

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Id = uuid.UUID

// NotebookListOrder 控制 notebook 列表查询的排序；具体 SQL 由 store 决定。
type NotebookListOrder int

const (
	// NotebookListOrderLastActive 按最后活跃时间倒序（updated_at DESC，id DESC 兜底）。
	NotebookListOrderLastActive NotebookListOrder = iota
	// NotebookListOrderCreateTime 按创建时间正序（uuidv7 id ASC）。
	NotebookListOrderCreateTime
)

type NotebookStore interface {
	Create(ctx context.Context, notebook *schema.Notebook) error
	Upsert(ctx context.Context, notebook *schema.Notebook) error
	GetById(ctx context.Context, id Id) (*schema.Notebook, error)
	Exists(ctx context.Context, id Id) (bool, error)
	GetByNameAndOwnerId(ctx context.Context, name string, ownerId valobj.Uid) (*schema.Notebook, error)
	// ListByOwnerId 分页返回 owner 的 notebook；excludeIds 非空时 NOT IN。
	ListByOwnerId(
		ctx context.Context,
		ownerId valobj.Uid,
		limit, offset int,
		order NotebookListOrder,
		excludeIds []Id,
	) ([]*schema.Notebook, error)
	// ListByIds 按 id 批量查询；ids 为空时返回空切片。
	ListByIds(ctx context.Context, ids []Id) ([]*schema.Notebook, error)
	// ListByIdsExcludingOwner 分页返回 ids 中 owner_id != excludeOwnerId 的 notebook；ids 为空时返回空切片。
	ListByIdsExcludingOwner(
		ctx context.Context,
		ids []Id,
		excludeOwnerId valobj.Uid,
		limit, offset int,
		order NotebookListOrder,
	) ([]*schema.Notebook, error)
	Update(ctx context.Context, notebook *schema.Notebook) error
	DeleteById(ctx context.Context, id Id) error
	UpdateName(ctx context.Context, params *schema.NotebookUpdateNameParams) error
	UpdateDescription(ctx context.Context, params *schema.NotebookUpdateDescriptionParams) error
	// 仅填充空的name/description，非空字段保持不变
	FillNameAndDescriptionIfEmpty(ctx context.Context, params *schema.NotebookFillNameAndDescriptionParams) error
}

// NotebookMaintenanceStore notebook 维护写操作（如 initjob 存量清洗），与主 Store 分离。
type NotebookMaintenanceStore interface {
	CountNonEmptyDescriptions(ctx context.Context) (int64, error)
	ClearNonEmptyDescriptions(ctx context.Context, params *schema.NotebookClearDescriptionsParams) error
}

type NotebookMemberStore interface {
	Upsert(ctx context.Context, member *schema.NotebookMember) error
	GetByNotebookIdAndUserId(ctx context.Context, notebookId Id, userId valobj.Uid) (*schema.NotebookMember, error)
	ListByNotebookId(ctx context.Context, notebookId Id) ([]*schema.NotebookMember, error)
	ListByUserId(ctx context.Context, userId valobj.Uid) ([]*schema.NotebookMember, error)
	Delete(ctx context.Context, notebookId Id, userId valobj.Uid) error
	DeleteByNotebookId(ctx context.Context, notebookId Id) error
}

type UserNotebookPinStore interface {
	Upsert(ctx context.Context, pin *schema.UserNotebookPin) error
	Get(ctx context.Context, userId valobj.Uid, notebookId Id) (*schema.UserNotebookPin, error)
	ListByUserId(ctx context.Context, userId valobj.Uid) ([]*schema.UserNotebookPin, error)
	CountByUserId(ctx context.Context, userId valobj.Uid) (int64, error)
	Delete(ctx context.Context, userId valobj.Uid, notebookId Id) error
	DeleteByNotebookId(ctx context.Context, notebookId Id) error
}

type SourceStore interface {
	Create(ctx context.Context, source *schema.Source) error
	UpdateExisting(ctx context.Context, source *schema.Source) (int64, error)
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
	GetByNotebookIdAndOwnerId(ctx context.Context, notebookId Id, ownerId valobj.Uid) (*schema.Chat, error)
	ListByNotebookId(ctx context.Context, notebookId Id) ([]*schema.Chat, error)
	ListByOwnerId(ctx context.Context, ownerId valobj.Uid, limit, offset int) ([]*schema.Chat, error)
	DeleteById(ctx context.Context, id Id) error
	DeleteByNotebookId(ctx context.Context, notebookId Id) error
}

type ChatMessageStore interface {
	Create(ctx context.Context, message *schema.ChatMessage) error
	GetById(ctx context.Context, id Id) (*schema.ChatMessage, error)
	GetByIdAndChatId(ctx context.Context, id Id, chatId Id) (*schema.ChatMessage, error)
	// 按照seqno从大到小排序, orderBy=0 -> default, orderBy=1 -> seq_no ASC
	ListByChatId(ctx context.Context, chatId Id, limit, offset, orderBy int) ([]*schema.ChatMessage, error)
	// 按照seqno从大到小排序, 查询seq_no < beforeSeqNo的消息
	ListByChatIdBeforeSeqNo(ctx context.Context, chatId Id, beforeSeqNo int64, limit int) ([]*schema.ChatMessage, error)
	DeleteByChatId(ctx context.Context, chatId Id) error
	BatchDeleteByChatIds(ctx context.Context, chatIds []Id) error
}

type ArtifactStore interface {
	Create(ctx context.Context, artifact *schema.Artifact) error
	UpdateExisting(ctx context.Context, artifact *schema.Artifact) (int64, error)
	GetById(ctx context.Context, id Id) (*schema.Artifact, error)
	ListByNotebookId(ctx context.Context, notebookId Id, limit, offset int) ([]*schema.Artifact, error)
	ListByStatus(ctx context.Context, statuses []string, limit int) ([]*schema.Artifact, error)
	DeleteById(ctx context.Context, id Id) error
	DeleteByNotebookId(ctx context.Context, notebookId Id) error
}

type ArtifactStylePreviewStore interface {
	Upsert(ctx context.Context, preview *schema.ArtifactStylePreview) error
	GetByIdentifier(ctx context.Context, identifier string) (*schema.ArtifactStylePreview, error)
	ListByIdentifiers(ctx context.Context, identifiers []string) ([]*schema.ArtifactStylePreview, error)
	DeleteByIdentifier(ctx context.Context, identifier string) error
}

type WorkerCheckpointStore interface {
	Create(ctx context.Context, cp *schema.WorkerCheckpoint) error
	GetByArtifactId(ctx context.Context, artifactId Id) (*schema.WorkerCheckpoint, error)
	Update(ctx context.Context, cp *schema.WorkerCheckpoint) error
	DeleteByArtifactId(ctx context.Context, artifactId Id) error
}

type UserStore interface {
	Create(ctx context.Context, user *schema.User) error
	GetById(ctx context.Context, id valobj.Uid) (*schema.User, error)
	BatchGetById(ctx context.Context, ids []valobj.Uid) ([]*schema.User, error)
	GetByEmail(ctx context.Context, email string) (*schema.User, error)
	GetPasswordHash(ctx context.Context, id valobj.Uid) (string, error)
	UpdateProfile(ctx context.Context, params *schema.UserUpdateProfileParams) error
	UpdateAvatar(ctx context.Context, params *schema.UserUpdateAvatarParams) error
	UpdatePasswordHash(ctx context.Context, params *schema.UserUpdatePasswordParams) error
}

type UserIdentityStore interface {
	Create(ctx context.Context, identity *schema.UserIdentity) error
	GetByProviderAndSubject(ctx context.Context, provider, subject string) (*schema.UserIdentity, error)
	// GetByUserIdAndId 按 (账号, 身份 id) 查询
	GetByUserIdAndId(ctx context.Context, userId valobj.Uid, id valobj.Id) (*schema.UserIdentity, error)
	ListByUserId(ctx context.Context, userId valobj.Uid) ([]*schema.UserIdentity, error)
	UpdateLastLoginAt(ctx context.Context, id valobj.Id, lastLoginAt int64) error
	Delete(ctx context.Context, id valobj.Id) error
	DeleteByUserId(ctx context.Context, userId valobj.Uid) error
}

// InitJobStore stores initjob run and task rows
// the adapter in internal/infrastructure/repository maps rows to initjob.Recorder
type InitJobStore interface {
	AbortStale(ctx context.Context, params *schema.InitJobAbortStaleParams) (
		abortedRuns int64, abortedTasks int64, err error,
	)
	CreateRun(ctx context.Context, run *schema.InitJobRun) error
	FinishRun(ctx context.Context, params *schema.InitJobRunFinishParams) error
	ListSucceededTaskIDs(ctx context.Context) ([]string, error)
	UpsertTaskRunning(ctx context.Context, task *schema.InitJobTask) error
	UpdateTaskStatus(ctx context.Context, params *schema.InitJobTaskStatusParams) error
}

type Dao struct {
	Closer misc.Closer

	NotebookStore             NotebookStore
	NotebookMaintenanceStore  NotebookMaintenanceStore
	NotebookMemberStore       NotebookMemberStore
	UserNotebookPinStore      UserNotebookPinStore
	SourceStore               SourceStore
	ChatStore                 ChatStore
	ChatMessageStore          ChatMessageStore
	ArtifactStore             ArtifactStore
	ArtifactStylePreviewStore ArtifactStylePreviewStore
	WorkerCheckpointStore     WorkerCheckpointStore
	InitJobStore              InitJobStore
	UserStore                 UserStore
	UserIdentityStore         UserIdentityStore
	Transactor                Transactor
}

func NewDao(
	closer misc.Closer,
	notebookStore NotebookStore,
	notebookMaintenanceStore NotebookMaintenanceStore,
	notebookMemberStore NotebookMemberStore,
	userNotebookPinStore UserNotebookPinStore,
	sourceStore SourceStore,
	chatStore ChatStore,
	chatMessageStore ChatMessageStore,
	artifactStore ArtifactStore,
	artifactStylePreviewStore ArtifactStylePreviewStore,
	workerCheckpointStore WorkerCheckpointStore,
	initJobStore InitJobStore,
	userStore UserStore,
	userIdentityStore UserIdentityStore,
	transactor Transactor,
) *Dao {
	return &Dao{
		Closer:                    closer,
		NotebookStore:             notebookStore,
		NotebookMaintenanceStore:  notebookMaintenanceStore,
		NotebookMemberStore:       notebookMemberStore,
		UserNotebookPinStore:      userNotebookPinStore,
		SourceStore:               sourceStore,
		ChatStore:                 chatStore,
		ChatMessageStore:          chatMessageStore,
		ArtifactStore:             artifactStore,
		ArtifactStylePreviewStore: artifactStylePreviewStore,
		WorkerCheckpointStore:     workerCheckpointStore,
		InitJobStore:              initJobStore,
		UserStore:                 userStore,
		UserIdentityStore:         userIdentityStore,
		Transactor:                transactor,
	}
}

func (d *Dao) Close(ctx context.Context) error {
	slog.WarnContext(ctx, "closing database connections...")
	return d.Closer.Close(ctx)
}
