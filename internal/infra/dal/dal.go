package dal

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/misc"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Id = uuid.UUID

type NotebookStore interface {
	Create(ctx context.Context, notebook *schema.Notebook) error
	GetById(ctx context.Context, id Id) (*schema.Notebook, error)
	GetByNameAndOwnerId(ctx context.Context, name, ownerId string) (*schema.Notebook, error)
	ListByOwnerId(ctx context.Context, ownerId string, limit, offset int) ([]*schema.Notebook, error)
	Update(ctx context.Context, notebook *schema.Notebook) error
	DeleteById(ctx context.Context, id Id) error
	UpdateName(ctx context.Context, id Id, name string) error
	UpdateDesc(ctx context.Context, id Id, desc string) error
}

type SourceStore interface {
	Create(ctx context.Context, source *schema.Source) error
	GetById(ctx context.Context, id Id) (*schema.Source, error)
	CountByNotebookId(ctx context.Context, notebookId Id) (int64, error)
	ListByNotebookId(ctx context.Context, notebookId Id, limit, offset int) ([]*schema.Source, error)
	DeleteById(ctx context.Context, id Id) error
	DeleteByNotebookId(ctx context.Context, notebookId Id) error
	UpdateStatus(ctx context.Context, id Id, status string) error
	Update(ctx context.Context, params *schema.SourceUpdateParams) error
	ListByIds(ctx context.Context, ids []Id) ([]*schema.Source, error)
	ListByNotebookIdAndIds(ctx context.Context, notebookId Id, ids []Id) ([]*schema.Source, error)
}

type ChatMessageStore interface {
	Create(ctx context.Context, message *schema.ChatMessage) error
	ListByChatId(ctx context.Context, chatId Id, limit, offset int) ([]*schema.ChatMessage, error)
	DeleteByChatId(ctx context.Context, chatId Id) error
}

type DAL struct {
	Closer misc.Closer

	NotebookStore    NotebookStore
	SourceStore      SourceStore
	ChatMessageStore ChatMessageStore
}

func (d *DAL) Close(ctx context.Context) error {
	slog.WarnContext(ctx, "closing database connections...")
	return d.Closer.Close(ctx)
}
