package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type StylePreviewRepository interface {
	Save(ctx context.Context, preview *entity.StylePreview) error
	// ErrStylePreviewNotFound is returned when a style preview is not found.
	FindByIdentifier(ctx context.Context, identifier entity.StylePreviewIdentifier) (*entity.StylePreview, error)
	ListByIdentifiers(ctx context.Context, identifiers []entity.StylePreviewIdentifier) ([]*entity.StylePreview, error)
	DeleteByIdentifier(ctx context.Context, identifier entity.StylePreviewIdentifier) error
}
