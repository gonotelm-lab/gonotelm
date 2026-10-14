package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UserIdentityRepositoryImpl struct {
	identityStore database.UserIdentityStore
}

var _ identityrepo.UserIdentityRepository = &UserIdentityRepositoryImpl{}

func NewUserIdentityRepository(identityStore database.UserIdentityStore) identityrepo.UserIdentityRepository {
	return &UserIdentityRepositoryImpl{identityStore: identityStore}
}

func (r *UserIdentityRepositoryImpl) Create(
	ctx context.Context,
	identity *identityentity.UserIdentity,
) error {
	if err := r.identityStore.Create(ctx, mapper.UserIdentityToSchema(identity)); err != nil {
		return errors.WithMessage(err, "failed to create user identity")
	}

	return nil
}

func (r *UserIdentityRepositoryImpl) GetByProviderAndSubject(
	ctx context.Context,
	provider identityentity.ProviderType,
	subject string,
) (*identityentity.UserIdentity, error) {
	sch, err := r.identityStore.GetByProviderAndSubject(ctx, provider.String(), subject)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identerr.ErrIdentityNotFound
		}

		return nil, errors.WithMessage(err, "failed to get user identity")
	}

	return mapper.UserIdentityFromSchema(sch), nil
}

func (r *UserIdentityRepositoryImpl) GetByUserIdAndId(
	ctx context.Context,
	userId valobj.Uid,
	id valobj.Id,
) (*identityentity.UserIdentity, error) {
	sch, err := r.identityStore.GetByUserIdAndId(ctx, userId, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identerr.ErrIdentityNotFound
		}

		return nil, errors.WithMessage(err, "failed to get user identity")
	}

	return mapper.UserIdentityFromSchema(sch), nil
}

func (r *UserIdentityRepositoryImpl) ListByUserId(
	ctx context.Context,
	userId valobj.Uid,
) ([]*identityentity.UserIdentity, error) {
	rows, err := r.identityStore.ListByUserId(ctx, userId)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to list user identities")
	}

	identities := make([]*identityentity.UserIdentity, 0, len(rows))
	for _, row := range rows {
		identities = append(identities, mapper.UserIdentityFromSchema(row))
	}

	return identities, nil
}

func (r *UserIdentityRepositoryImpl) TouchLogin(
	ctx context.Context,
	identity *identityentity.UserIdentity,
) error {
	err := r.identityStore.UpdateLastLoginAt(ctx, identity.Id, identity.LastLoginAt.Value())
	if err != nil {
		return errors.WithMessage(err, "failed to touch identity login time")
	}

	return nil
}

func (r *UserIdentityRepositoryImpl) Delete(ctx context.Context, id valobj.Id) error {
	if err := r.identityStore.Delete(ctx, id); err != nil {
		return errors.WithMessage(err, "failed to delete user identity")
	}

	return nil
}

func (r *UserIdentityRepositoryImpl) DeleteByUserId(ctx context.Context, userId valobj.Uid) error {
	if err := r.identityStore.DeleteByUserId(ctx, userId); err != nil {
		return errors.WithMessage(err, "failed to delete user identities")
	}

	return nil
}
