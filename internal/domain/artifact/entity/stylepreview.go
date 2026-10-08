package entity

import (
	"slices"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type StylePreviewIdentifier string

func (i StylePreviewIdentifier) String() string {
	return string(i)
}

const (
	StylePreviewSlidesDefault     StylePreviewIdentifier = "slides.style.default"
	StylePreviewSlidesCute        StylePreviewIdentifier = "slides.style.cute"
	StylePreviewSlidesEducational StylePreviewIdentifier = "slides.style.educational"

	StylePreviewInfoGraphicDefault     StylePreviewIdentifier = "info_graphic.style.default"
	StylePreviewInfoGraphicHandDrawn   StylePreviewIdentifier = "info_graphic.style.hand-drawn"
	StylePreviewInfoGraphicAnime       StylePreviewIdentifier = "info_graphic.style.anime"
	StylePreviewInfoGraphicCute        StylePreviewIdentifier = "info_graphic.style.cute"
	StylePreviewInfoGraphicEducational StylePreviewIdentifier = "info_graphic.style.educational"
	StylePreviewInfoGraphicMinimal25D  StylePreviewIdentifier = "info_graphic.style.minimal-2.5d"

	StylePreviewVideoOverviewDefault     StylePreviewIdentifier = "video_overview.style.default"
	StylePreviewVideoOverviewCute        StylePreviewIdentifier = "video_overview.style.cute"
	StylePreviewVideoOverviewEducational StylePreviewIdentifier = "video_overview.style.educational"
)

type StylePreviewRef struct {
	// VisualStyle 为 visual_style 接受的值
	VisualStyle string
	Identifier  StylePreviewIdentifier
}

var slidesStylePreviews = []StylePreviewRef{
	{VisualStyle: SlidesVisualStyleDefault.String(), Identifier: StylePreviewSlidesDefault},
	{VisualStyle: SlidesVisualStyleCute.String(), Identifier: StylePreviewSlidesCute},
	{VisualStyle: SlidesVisualStyleEducational.String(), Identifier: StylePreviewSlidesEducational},
}

var infoGraphicStylePreviews = []StylePreviewRef{
	{VisualStyle: InfoGraphicVisualStyleDefault.String(), Identifier: StylePreviewInfoGraphicDefault},
	{VisualStyle: InfoGraphicVisualStyleHandDrawn.String(), Identifier: StylePreviewInfoGraphicHandDrawn},
	{VisualStyle: InfoGraphicVisualStyleAnime.String(), Identifier: StylePreviewInfoGraphicAnime},
	{VisualStyle: InfoGraphicVisualStyleCute.String(), Identifier: StylePreviewInfoGraphicCute},
	{VisualStyle: InfoGraphicVisualStyleEducational.String(), Identifier: StylePreviewInfoGraphicEducational},
	{VisualStyle: InfoGraphicVisualStyleMinimal25D.String(), Identifier: StylePreviewInfoGraphicMinimal25D},
}

var videoOverviewStylePreviews = []StylePreviewRef{
	{VisualStyle: VideoOverviewStyleDefault.String(), Identifier: StylePreviewVideoOverviewDefault},
	{VisualStyle: VideoOverviewStyleCute.String(), Identifier: StylePreviewVideoOverviewCute},
	{VisualStyle: VideoOverviewStyleEducational.String(), Identifier: StylePreviewVideoOverviewEducational},
}

// StylePreviewsForKind 返回该 kind 的风格列表
func StylePreviewsForKind(kind Kind) []StylePreviewRef {
	switch kind {
	case KindSlides:
		return slices.Clone(slidesStylePreviews)
	case KindInfoGraphic:
		return slices.Clone(infoGraphicStylePreviews)
	case KindVideoOverview:
		return slices.Clone(videoOverviewStylePreviews)
	default:
		return nil
	}
}

// DefaultVisualStyleForKind 返回该 kind 的默认风格
func DefaultVisualStyleForKind(kind Kind) string {
	switch kind {
	case KindSlides:
		return SlidesVisualStyleDefaultValue().String()
	case KindInfoGraphic:
		return InfoGraphicVisualStyleDefault.String()
	case KindVideoOverview:
		return VideoOverviewStyleDefaultValue().String()
	default:
		return ""
	}
}

type StylePreview struct {
	StoreKey         valobj.StoreKey
	Identifier       StylePreviewIdentifier
	Md5sum           []byte
	OriginalFilename string
	CreateTime       valobj.Time
	UpdateTime       valobj.Time
}

func NewStylePreview(identifier StylePreviewIdentifier, storeKey valobj.StoreKey) *StylePreview {
	now := valobj.NewTime()
	return &StylePreview{
		StoreKey:   storeKey,
		Identifier: identifier,
		CreateTime: now,
		UpdateTime: now,
	}
}

func (p *StylePreview) UpdateAsset(storeKey valobj.StoreKey, md5sum []byte, originalFilename string) {
	p.StoreKey = storeKey
	p.Md5sum = md5sum
	p.OriginalFilename = originalFilename
	p.UpdateTime = valobj.NewTime()
}
