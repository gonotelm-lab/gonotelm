package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema/mapper"
)

type ContextMessageRepositoryImpl struct {
	contextCache cache.ChatContextMessageCache
}

func NewContextMessageRepository(contextCache cache.ChatContextMessageCache) chatrepo.ContextMessageRepository {
	return &ContextMessageRepositoryImpl{
		contextCache: contextCache,
	}
}

var _ chatrepo.ContextMessageRepository = &ContextMessageRepositoryImpl{}

func (r *ContextMessageRepositoryImpl) Append(
	ctx context.Context,
	chatId valobj.Id,
	messages []*entity.ContextMessage,
) error {
	schemas, err := mapper.ContextMessagesToSchema(messages)
	if err != nil {
		return err
	}

	return r.contextCache.Append(ctx, chatId.String(), schemas)
}

func (r *ContextMessageRepositoryImpl) Destroy(ctx context.Context, chatId valobj.Id) error {
	return r.contextCache.Destroy(ctx, chatId.String())
}

func (r *ContextMessageRepositoryImpl) BatchDestroy(ctx context.Context, chatIds []valobj.Id) error {
	if len(chatIds) == 0 {
		return nil
	}

	ids := make([]string, 0, len(chatIds))
	for _, chatId := range chatIds {
		ids = append(ids, chatId.String())
	}

	return r.contextCache.BatchDestroy(ctx, ids)
}

func (r *ContextMessageRepositoryImpl) ListAll(
	ctx context.Context,
	chatId valobj.Id,
) ([]*entity.ContextMessage, error) {
	schemas, err := r.contextCache.ListAll(ctx, chatId.String())
	if err != nil {
		return nil, err
	}

	return mapper.ContextMessagesFromSchema(schemas)
}

func (r *ContextMessageRepositoryImpl) Set(
	ctx context.Context,
	chatId valobj.Id,
	messages []*entity.ContextMessage,
) error {
	schemas, err := mapper.ContextMessagesToSchema(messages)
	if err != nil {
		return err
	}

	return r.contextCache.Override(ctx, chatId.String(), schemas)
}
