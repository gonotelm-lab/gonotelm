package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	pinerrors "github.com/gonotelm-lab/gonotelm/internal/domain/pin/errors"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type PinRepositoryImpl struct {
	pinStore database.UserNotebookPinStore
}

func NewPinRepository(pinStore database.UserNotebookPinStore) pinrepo.Repository {
	return &PinRepositoryImpl{pinStore: pinStore}
}

var _ pinrepo.Repository = &PinRepositoryImpl{}

func (r *PinRepositoryImpl) Save(
	ctx context.Context,
	pin *pinentity.UserNotebookPin,
) error {
	return r.pinStore.Upsert(ctx, mapper.UserNotebookPinToSchema(pin))
}

func (r *PinRepositoryImpl) Find(
	ctx context.Context,
	userId valobj.Uid,
	notebookId valobj.Id,
) (*pinentity.UserNotebookPin, error) {
	row, err := r.pinStore.Get(ctx, userId, notebookId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, pinerrors.ErrPinNotFound
		}
		return nil, err
	}

	return mapper.UserNotebookPinFromSchema(row), nil
}

func (r *PinRepositoryImpl) ListByUserId(
	ctx context.Context,
	userId valobj.Uid,
) ([]*pinentity.UserNotebookPin, error) {
	rows, err := r.pinStore.ListByUserId(ctx, userId)
	if err != nil {
		return nil, err
	}

	return mapper.UserNotebookPinsFromSchema(rows), nil
}

func (r *PinRepositoryImpl) CountByUserId(
	ctx context.Context,
	userId valobj.Uid,
) (int64, error) {
	return r.pinStore.CountByUserId(ctx, userId)
}

func (r *PinRepositoryImpl) Delete(
	ctx context.Context,
	userId valobj.Uid,
	notebookId valobj.Id,
) error {
	return r.pinStore.Delete(ctx, userId, notebookId)
}

func (r *PinRepositoryImpl) DeleteByNotebookId(
	ctx context.Context,
	notebookId valobj.Id,
) error {
	return r.pinStore.DeleteByNotebookId(ctx, notebookId)
}
