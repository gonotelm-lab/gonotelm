package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
)

type SuggestionRepositoryImpl struct {
	suggestionCache cache.ChatSuggestionCache
}

func NewSuggestionRepository(suggestionCache cache.ChatSuggestionCache) chatrepo.SuggestionRepository {
	return &SuggestionRepositoryImpl{
		suggestionCache: suggestionCache,
	}
}

var _ chatrepo.SuggestionRepository = &SuggestionRepositoryImpl{}

func (r *SuggestionRepositoryImpl) Get(ctx context.Context, chatId valobj.Id) (*entity.Suggestion, error) {
	sch, err := r.suggestionCache.Get(ctx, chatId.String())
	if err != nil {
		return nil, err
	}
	if sch == nil {
		return entity.NewSuggestion(entity.SuggestionTypeNone, []string{}), nil // 不存在也无所谓
	}

	return mapper.SuggestionFromSchema(sch), nil
}

func (r *SuggestionRepositoryImpl) Save(ctx context.Context, chatId valobj.Id, suggestion *entity.Suggestion) error {
	return r.suggestionCache.Set(ctx, chatId.String(), mapper.SuggestionToSchema(suggestion))
}

func (r *SuggestionRepositoryImpl) Delete(ctx context.Context, chatId valobj.Id) error {
	return r.suggestionCache.Delete(ctx, chatId.String())
}
