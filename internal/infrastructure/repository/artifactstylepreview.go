package repository

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type StylePreviewRepositoryImpl struct {
	store database.ArtifactStylePreviewStore
	cache cache.StylePreviewCache
}

func NewStylePreviewRepository(
	store database.ArtifactStylePreviewStore,
	previewCache cache.StylePreviewCache,
) artifactrepo.StylePreviewRepository {
	return &StylePreviewRepositoryImpl{store: store, cache: previewCache}
}

var _ artifactrepo.StylePreviewRepository = &StylePreviewRepositoryImpl{}

func (r *StylePreviewRepositoryImpl) Save(ctx context.Context, p *entity.StylePreview) error {
	if err := r.store.Upsert(ctx, mapper.StylePreviewToSchema(p)); err != nil {
		return err
	}

	r.setCache(ctx, p)

	return nil
}

func (r *StylePreviewRepositoryImpl) FindByIdentifier(
	ctx context.Context, identifier entity.StylePreviewIdentifier,
) (*entity.StylePreview, error) {
	if p, ok := r.getCache(ctx, identifier); ok {
		return p, nil
	}

	sch, err := r.store.GetByIdentifier(ctx, identifier.String())
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, artifacterrors.ErrStylePreviewNotFound
		}
		return nil, err
	}

	p, err := mapper.StylePreviewFromSchema(sch)
	if err != nil {
		return nil, err
	}

	r.setCache(ctx, p)

	return p, nil
}

func (r *StylePreviewRepositoryImpl) ListByIdentifiers(
	ctx context.Context, identifiers []entity.StylePreviewIdentifier,
) ([]*entity.StylePreview, error) {
	ids := make([]string, 0, len(identifiers))
	for _, id := range identifiers {
		ids = append(ids, id.String())
	}
	if len(ids) == 0 {
		return []*entity.StylePreview{}, nil
	}

	cached := r.getCacheMulti(ctx, ids)

	missing := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := cached[id]; !ok {
			missing = append(missing, id)
		}
	}

	loaded := make(map[string]*entity.StylePreview, len(missing))
	if len(missing) > 0 {
		rows, err := r.store.ListByIdentifiers(ctx, missing)
		if err != nil {
			return nil, err
		}

		previews := make([]*entity.StylePreview, 0, len(rows))
		for _, row := range rows {
			p, err := mapper.StylePreviewFromSchema(row)
			if err != nil {
				return nil, err
			}
			loaded[p.Identifier.String()] = p
			previews = append(previews, p)
		}

		r.setCacheMulti(ctx, previews)
	}

	out := make([]*entity.StylePreview, 0, len(ids))
	for _, id := range ids {
		if p, ok := cached[id]; ok {
			out = append(out, p)
			continue
		}
		if p, ok := loaded[id]; ok {
			out = append(out, p)
		}
	}

	return out, nil
}

func (r *StylePreviewRepositoryImpl) DeleteByIdentifier(ctx context.Context, identifier entity.StylePreviewIdentifier) error {
	if err := r.store.DeleteByIdentifier(ctx, identifier.String()); err != nil {
		return err
	}

	if r.cache != nil {
		if err := r.cache.Delete(ctx, identifier.String()); err != nil {
			slog.WarnContext(ctx, "delete style preview cache failed",
				slog.String("identifier", identifier.String()),
				slog.Any("err", err),
			)
		}
	}

	return nil
}

// getCache 只有命中且解码成功才返回 true，其余情况（未配置缓存、缓存故障、
// 数据损坏）都交给调用方回源。
func (r *StylePreviewRepositoryImpl) getCache(
	ctx context.Context, identifier entity.StylePreviewIdentifier,
) (*entity.StylePreview, bool) {
	if r.cache == nil {
		return nil, false
	}

	sch, err := r.cache.Get(ctx, identifier.String())
	if err != nil {
		slog.WarnContext(ctx, "get style preview from cache failed, fallback to store",
			slog.String("identifier", identifier.String()),
			slog.Any("err", err),
		)
		return nil, false
	}
	if sch == nil {
		return nil, false
	}

	p, err := mapper.StylePreviewFromCacheSchema(sch)
	if err != nil {
		slog.WarnContext(ctx, "decode cached style preview failed, fallback to store",
			slog.String("identifier", identifier.String()),
			slog.Any("err", err),
		)
		return nil, false
	}

	return p, true
}

func (r *StylePreviewRepositoryImpl) getCacheMulti(
	ctx context.Context, ids []string,
) map[string]*entity.StylePreview {
	out := make(map[string]*entity.StylePreview, len(ids))
	if r.cache == nil {
		return out
	}

	schemas, err := r.cache.GetMulti(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "get style previews from cache failed, fallback to store",
			slog.Int("count", len(ids)),
			slog.Any("err", err),
		)
		return out
	}

	for id, sch := range schemas {
		p, err := mapper.StylePreviewFromCacheSchema(sch)
		if err != nil {
			slog.WarnContext(ctx, "decode cached style preview failed, fallback to store",
				slog.String("identifier", id),
				slog.Any("err", err),
			)
			continue
		}
		out[id] = p
	}

	return out
}

func (r *StylePreviewRepositoryImpl) setCache(ctx context.Context, p *entity.StylePreview) {
	if r.cache == nil {
		return
	}

	if err := r.cache.Set(ctx, p.Identifier.String(), mapper.StylePreviewToCacheSchema(p)); err != nil {
		slog.WarnContext(ctx, "cache style preview failed",
			slog.String("identifier", p.Identifier.String()),
			slog.Any("err", err),
		)
	}
}

func (r *StylePreviewRepositoryImpl) setCacheMulti(ctx context.Context, previews []*entity.StylePreview) {
	if r.cache == nil || len(previews) == 0 {
		return
	}

	if err := r.cache.SetMulti(ctx, mapper.StylePreviewsToCacheSchema(previews)); err != nil {
		slog.WarnContext(ctx, "cache style previews failed",
			slog.Int("count", len(previews)),
			slog.Any("err", err),
		)
	}
}
