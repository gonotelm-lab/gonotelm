package stylepreview

import "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

// Asset 一项预览资产：本地文件名 + 它落库用的 identifier。
type Asset struct {
	FileName   string
	Identifier entity.StylePreviewIdentifier
}

// Provider 描述一个 kind 的预览资产。文件名与路径只在这一层出现，不进 domain：
// domain 只维护 identifier ↔ 风格值的词表，供接口和生成校验使用。
type Provider interface {
	Kind() entity.Kind
	LocalDir() string
	KeyPrefix() string
	Assets() []Asset
}

type slidesProvider struct{}

func SlidesProvider() Provider { return slidesProvider{} }

func (slidesProvider) Kind() entity.Kind { return entity.KindSlides }

func (slidesProvider) LocalDir() string { return "artifact-preview/slides" }

func (slidesProvider) KeyPrefix() string { return "artifact-preview/slides/v1" }

func (slidesProvider) Assets() []Asset {
	return []Asset{
		{FileName: "cute-v1.webp", Identifier: entity.StylePreviewSlidesCute},
		{FileName: "default-v1.webp", Identifier: entity.StylePreviewSlidesDefault},
		{FileName: "educational-v1.webp", Identifier: entity.StylePreviewSlidesEducational},
	}
}

type infoGraphicProvider struct{}

func InfoGraphicProvider() Provider { return infoGraphicProvider{} }

func (infoGraphicProvider) Kind() entity.Kind { return entity.KindInfoGraphic }

func (infoGraphicProvider) LocalDir() string { return "artifact-preview/infographic" }

func (infoGraphicProvider) KeyPrefix() string { return "artifact-preview/infographic/v1" }

func (infoGraphicProvider) Assets() []Asset {
	return []Asset{
		{FileName: "anime-v1.webp", Identifier: entity.StylePreviewInfoGraphicAnime},
		{FileName: "cute-v1.webp", Identifier: entity.StylePreviewInfoGraphicCute},
		{FileName: "default-v1.webp", Identifier: entity.StylePreviewInfoGraphicDefault},
		{FileName: "educational-v1.webp", Identifier: entity.StylePreviewInfoGraphicEducational},
		{FileName: "hand-drawn-v1.webp", Identifier: entity.StylePreviewInfoGraphicHandDrawn},
		{FileName: "minimal-2.5d-v1.webp", Identifier: entity.StylePreviewInfoGraphicMinimal25D},
	}
}

type videoOverviewProvider struct{}

func VideoOverviewProvider() Provider { return videoOverviewProvider{} }

func (videoOverviewProvider) Kind() entity.Kind { return entity.KindVideoOverview }

func (videoOverviewProvider) LocalDir() string { return "artifact-preview/video-overview" }

func (videoOverviewProvider) KeyPrefix() string { return "artifact-preview/video-overview/v1" }

func (videoOverviewProvider) Assets() []Asset {
	return []Asset{
		{FileName: "cute-v1.webp", Identifier: entity.StylePreviewVideoOverviewCute},
		{FileName: "default-v1.webp", Identifier: entity.StylePreviewVideoOverviewDefault},
		{FileName: "educational-v1.webp", Identifier: entity.StylePreviewVideoOverviewEducational},
	}
}
