package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
)

type NotebookMemberRepositoryImpl struct {
	memberStore database.NotebookMemberStore
}

func NewNotebookMemberRepository(
	memberStore database.NotebookMemberStore,
) notebookrepo.MemberRepository {
	return &NotebookMemberRepositoryImpl{memberStore: memberStore}
}

var _ notebookrepo.MemberRepository = &NotebookMemberRepositoryImpl{}

func (r *NotebookMemberRepositoryImpl) Save(
	ctx context.Context,
	member *notebookentity.NotebookMember,
) error {
	return r.memberStore.Upsert(ctx, mapper.NotebookMemberToSchema(member))
}

func (r *NotebookMemberRepositoryImpl) FindByNotebookIdAndUserId(
	ctx context.Context,
	notebookId valobj.Id,
	userId valobj.Uid,
) (*notebookentity.NotebookMember, error) {
	row, err := r.memberStore.GetByNotebookIdAndUserId(ctx, notebookId, userId)
	if err != nil {
		return nil, err
	}

	return mapper.NotebookMemberFromSchema(row), nil
}

func (r *NotebookMemberRepositoryImpl) ListByNotebookId(
	ctx context.Context,
	notebookId valobj.Id,
) ([]*notebookentity.NotebookMember, error) {
	rows, err := r.memberStore.ListByNotebookId(ctx, notebookId)
	if err != nil {
		return nil, err
	}

	return mapper.NotebookMembersFromSchema(rows), nil
}

func (r *NotebookMemberRepositoryImpl) ListByUserId(
	ctx context.Context,
	userId valobj.Uid,
) ([]*notebookentity.NotebookMember, error) {
	rows, err := r.memberStore.ListByUserId(ctx, userId)
	if err != nil {
		return nil, err
	}

	return mapper.NotebookMembersFromSchema(rows), nil
}

func (r *NotebookMemberRepositoryImpl) Delete(
	ctx context.Context,
	notebookId valobj.Id,
	userId valobj.Uid,
) error {
	return r.memberStore.Delete(ctx, notebookId, userId)
}

func (r *NotebookMemberRepositoryImpl) DeleteByNotebookId(
	ctx context.Context,
	notebookId valobj.Id,
) error {
	return r.memberStore.DeleteByNotebookId(ctx, notebookId)
}
