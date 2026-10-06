package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func StylePreviewToSchema(p *entity.StylePreview) *schema.ArtifactStylePreview {
	return &schema.ArtifactStylePreview{
		StoreKey:         p.StoreKey.Encode(),
		Identifier:       p.Identifier.String(),
		Md5sum:           p.Md5sum,
		OriginalFilename: p.OriginalFilename,
		CreatedAt:        p.CreateTime.Value(),
		UpdatedAt:        p.UpdateTime.Value(),
	}
}

func StylePreviewFromSchema(sch *schema.ArtifactStylePreview) (*entity.StylePreview, error) {
	key, err := valobj.DecodeStoreKey(sch.StoreKey)
	if err != nil {
		return nil, err
	}

	return &entity.StylePreview{
		StoreKey:         key,
		Identifier:       entity.StylePreviewIdentifier(sch.Identifier),
		Md5sum:           sch.Md5sum,
		OriginalFilename: sch.OriginalFilename,
		CreateTime:       valobj.NewTimeFrom(sch.CreatedAt),
		UpdateTime:       valobj.NewTimeFrom(sch.UpdatedAt),
	}, nil
}
