package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type NotebookStoreImpl struct {
	db *gorm.DB
}

var _ dal.NotebookStore = &NotebookStoreImpl{}

func NewNotebookStoreImpl(db *gorm.DB) *NotebookStoreImpl {
	return &NotebookStoreImpl{db: db}
}

func (s *NotebookStoreImpl) Create(ctx context.Context, notebook *schema.Notebook) error {
	tx := s.db.WithContext(ctx).Exec(
		"INSERT INTO notebooks (id, name, description, owner_id, updated_at) VALUES (?, ?, ?, ?, ?)",
		notebook.Id, notebook.Name, notebook.Description, notebook.OwnerId, notebook.UpdatedAt,
	)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}

func (s *NotebookStoreImpl) GetById(ctx context.Context, id dal.Id) (*schema.Notebook, error) {
	notebook, err := gorm.G[*schema.Notebook](s.db).
		Raw(
			"SELECT id, name, description, owner_id, updated_at FROM notebooks WHERE id = ? LIMIT 1",
			id).
		First(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return notebook, nil
}

func (s *NotebookStoreImpl) GetByNameAndOwnerId(
	ctx context.Context,
	name, ownerId string,
) (*schema.Notebook, error) {
	notebook, err := gorm.G[*schema.Notebook](s.db).
		Raw("SELECT id, name, description, owner_id, updated_at FROM notebooks WHERE name = ? AND owner_id = ? LIMIT 1",
			name,
			ownerId,
		).
		First(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return notebook, nil
}

func (s *NotebookStoreImpl) ListByOwnerId(
	ctx context.Context,
	ownerId string,
	limit, offset int,
) ([]*schema.Notebook, error) {
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	rows, err := gorm.G[*schema.Notebook](s.db).
		Raw("SELECT id, name, description, owner_id, updated_at FROM notebooks WHERE owner_id = ? ORDER BY updated_at DESC LIMIT ? OFFSET ?",
			ownerId, limit, offset).
		Find(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *NotebookStoreImpl) Update(ctx context.Context, notebook *schema.Notebook) error {
	tx := s.db.WithContext(ctx).Exec(
		"UPDATE notebooks SET name = ?, description = ?, owner_id = ?, updated_at = ? WHERE id = ?",
		notebook.Name, notebook.Description, notebook.OwnerId, notebook.UpdatedAt, notebook.Id,
	)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}
	return nil
}

func (s *NotebookStoreImpl) DeleteById(ctx context.Context, id dal.Id) error {
	tx := s.db.WithContext(ctx).Exec(`DELETE FROM notebooks WHERE id = ?`, id)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}

func (s *NotebookStoreImpl) UpdateName(ctx context.Context, id dal.Id, name string) error {
	tx := s.db.WithContext(ctx).Exec("UPDATE notebooks SET name = ? WHERE id = ?", name, id)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}
	return nil
}

func (s *NotebookStoreImpl) UpdateDesc(ctx context.Context, id dal.Id, desc string) error {
	tx := s.db.WithContext(ctx).Exec("UPDATE notebooks SET description = ? WHERE id = ?", desc, id)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}
	return nil
}
