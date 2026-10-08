package artifact

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListStylePreviewsQuery struct {
	Kind entity.Kind
}

type StylePreviewItem struct {
	VisualStyle string
	// 图片未播种或解析失败时为空串。
	PreviewUrl string
}

type ListStylePreviewsResult struct {
	Kind               entity.Kind
	DefaultVisualStyle string
	Previews           []StylePreviewItem
}

type ListStylePreviewsHandler struct {
	stylePreviewRepo artifactrepo.StylePreviewRepository
	objectStore      adapter.ObjectPublicURLer
}

func NewListStylePreviewsHandler(
	stylePreviewRepo artifactrepo.StylePreviewRepository,
	objectStore adapter.ObjectPublicURLer,
) *ListStylePreviewsHandler {
	return &ListStylePreviewsHandler{stylePreviewRepo: stylePreviewRepo, objectStore: objectStore}
}

func (h *ListStylePreviewsHandler) Handle(
	ctx context.Context, cmd *ListStylePreviewsQuery,
) (*ListStylePreviewsResult, error) {
	refs := entity.StylePreviewsForKind(cmd.Kind)
	if len(refs) == 0 {
		return nil, artifacterrors.ErrInvalidKind.Msgf("artifact kind has no style previews: %s", cmd.Kind)
	}

	identifiers := make([]entity.StylePreviewIdentifier, 0, len(refs))
	for _, ref := range refs {
		identifiers = append(identifiers, ref.Identifier)
	}

	rows, err := h.stylePreviewRepo.ListByIdentifiers(ctx, identifiers)
	if err != nil {
		return nil, errors.WithMessagef(err, "list style previews failed, kind=%s", cmd.Kind)
	}

	stored := make(map[entity.StylePreviewIdentifier]*entity.StylePreview, len(rows))
	for _, row := range rows {
		stored[row.Identifier] = row
	}

	previews := make([]StylePreviewItem, 0, len(refs))
	for _, ref := range refs {
		previews = append(previews, StylePreviewItem{
			VisualStyle: ref.VisualStyle,
			PreviewUrl:  h.previewURL(ctx, cmd.Kind, ref, stored[ref.Identifier]),
		})
	}

	return &ListStylePreviewsResult{
		Kind:               cmd.Kind,
		DefaultVisualStyle: entity.DefaultVisualStyleForKind(cmd.Kind),
		Previews:           previews,
	}, nil
}

// 缺失或解析失败返回空串，不因此让整个请求失败。
func (h *ListStylePreviewsHandler) previewURL(
	ctx context.Context,
	kind entity.Kind,
	ref entity.StylePreviewRef,
	preview *entity.StylePreview,
) string {
	if preview == nil || !preview.StoreKey.Valid() {
		slog.WarnContext(ctx, "style preview is not seeded",
			slog.String("kind", kind.String()),
			slog.String("identifier", ref.Identifier.String()),
		)
		return ""
	}

	url, err := h.objectStore.PublicURL(ctx, preview.StoreKey)
	if err != nil {
		slog.WarnContext(ctx, "resolve style preview public url failed",
			slog.String("kind", kind.String()),
			slog.String("identifier", ref.Identifier.String()),
			slog.Any("err", err),
		)
		return ""
	}

	return url
}
