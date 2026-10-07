package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	dbschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func StylePreviewToSchema(p *entity.StylePreview) *dbschema.ArtifactStylePreview {
	return &dbschema.ArtifactStylePreview{
		StoreKey:         p.StoreKey.Encode(),
		Identifier:       p.Identifier.String(),
		Md5sum:           p.Md5sum,
		OriginalFilename: p.OriginalFilename,
		CreatedAt:        p.CreateTime.Value(),
		UpdatedAt:        p.UpdateTime.Value(),
	}
}

func StylePreviewFromSchema(sch *dbschema.ArtifactStylePreview) (*entity.StylePreview, error) {
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

func StylePreviewToCacheSchema(p *entity.StylePreview) *cacheschema.StylePreview {
	return &cacheschema.StylePreview{
		StoreKey:         p.StoreKey.Encode(),
		Identifier:       p.Identifier.String(),
		Md5sum:           p.Md5sum,
		OriginalFilename: p.OriginalFilename,
		CreateTime:       p.CreateTime.Value(),
		UpdateTime:       p.UpdateTime.Value(),
	}
}

func StylePreviewsToCacheSchema(previews []*entity.StylePreview) []*cacheschema.StylePreview {
	out := make([]*cacheschema.StylePreview, 0, len(previews))
	for _, p := range previews {
		out = append(out, StylePreviewToCacheSchema(p))
	}

	return out
}

func StylePreviewFromCacheSchema(sch *cacheschema.StylePreview) (*entity.StylePreview, error) {
	key, err := valobj.DecodeStoreKey(sch.StoreKey)
	if err != nil {
		return nil, err
	}

	return &entity.StylePreview{
		StoreKey:         key,
		Identifier:       entity.StylePreviewIdentifier(sch.Identifier),
		Md5sum:           sch.Md5sum,
		OriginalFilename: sch.OriginalFilename,
		CreateTime:       valobj.NewTimeFrom(sch.CreateTime),
		UpdateTime:       valobj.NewTimeFrom(sch.UpdateTime),
	}, nil
}
