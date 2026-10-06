package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ArtifactStylePreviewStoreImpl struct{ db *gorm.DB }

var _ database.ArtifactStylePreviewStore = &ArtifactStylePreviewStoreImpl{}

func NewArtifactStylePreviewStoreImpl(db *gorm.DB) *ArtifactStylePreviewStoreImpl {
	return &ArtifactStylePreviewStoreImpl{db: db}
}

func (s *ArtifactStylePreviewStoreImpl) Upsert(ctx context.Context, p *schema.ArtifactStylePreview) error {
	cl := clause.OnConflict{
		Columns: []clause.Column{{Name: "identifier"}},
		DoUpdates: clause.Assignments(map[string]any{
			"store_key":         p.StoreKey,
			"md5sum":            p.Md5sum,
			"original_filename": p.OriginalFilename,
			"updated_at":        p.UpdatedAt,
		}),
	}
	if err := s.db.WithContext(ctx).
		Model(&schema.ArtifactStylePreview{}).
		Clauses(cl).
		Create(p).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *ArtifactStylePreviewStoreImpl) GetByIdentifier(
	ctx context.Context, identifier string,
) (*schema.ArtifactStylePreview, error) {
	var p schema.ArtifactStylePreview
	if err := s.db.WithContext(ctx).
		Where("identifier = ?", identifier).
		Take(&p).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return &p, nil
}

func (s *ArtifactStylePreviewStoreImpl) ListByIdentifiers(
	ctx context.Context, identifiers []string,
) ([]*schema.ArtifactStylePreview, error) {
	if len(identifiers) == 0 {
		return nil, nil
	}

	var rows []*schema.ArtifactStylePreview
	if err := s.db.WithContext(ctx).
		Where("identifier IN ?", identifiers).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *ArtifactStylePreviewStoreImpl) DeleteByIdentifier(ctx context.Context, identifier string) error {
	if err := s.db.WithContext(ctx).
		Where("identifier = ?", identifier).
		Delete(&schema.ArtifactStylePreview{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
