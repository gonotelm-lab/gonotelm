package entity

type InfoGraphicOrientation string

const (
	InfoGraphicOrientationPortrait  InfoGraphicOrientation = "portrait"
	InfoGraphicOrientationLandscape InfoGraphicOrientation = "landscape"
	InfoGraphicOrientationSquare    InfoGraphicOrientation = "square"
)

func (o InfoGraphicOrientation) String() string { return string(o) }
func (o InfoGraphicOrientation) Supported() bool {
	switch o {
	case InfoGraphicOrientationPortrait,
		InfoGraphicOrientationLandscape,
		InfoGraphicOrientationSquare:
		return true
	}
	return false
}

func (o InfoGraphicOrientation) ImageSize() (int, int) {
	switch o {
	case InfoGraphicOrientationPortrait:
		return 720, 1280
	case InfoGraphicOrientationLandscape:
		return 1280, 720
	case InfoGraphicOrientationSquare:
		return 1024, 1024
	}
	return 1280, 720
}

func InfoGraphicOrientationDefault() InfoGraphicOrientation {
	return InfoGraphicOrientationLandscape
}

type InfoGraphicVisualStyle string

const (
	InfoGraphicVisualStyleDefault    InfoGraphicVisualStyle = "default"
	InfoGraphicVisualStyleHandDrawn  InfoGraphicVisualStyle = "hand-drawn"
	InfoGraphicVisualStyleAnime      InfoGraphicVisualStyle = "anime"
	InfoGraphicVisualStyleCute       InfoGraphicVisualStyle = "cute"
	InfoGraphicVisualStyleEducational InfoGraphicVisualStyle = "educational"
	InfoGraphicVisualStyleMinimal25D  InfoGraphicVisualStyle = "minimal-2.5d"
)

func (s InfoGraphicVisualStyle) String() string { return string(s) }
func (s InfoGraphicVisualStyle) Supported() bool {
	switch s {
	case InfoGraphicVisualStyleDefault,
		InfoGraphicVisualStyleHandDrawn,
		InfoGraphicVisualStyleAnime,
		InfoGraphicVisualStyleCute,
		InfoGraphicVisualStyleEducational,
		InfoGraphicVisualStyleMinimal25D:
		return true
	}
	return false
}

type InfoGraphicDetailLevel string

const (
	InfoGraphicDetailLevelConcise  InfoGraphicDetailLevel = "concise"
	InfoGraphicDetailLevelStandard InfoGraphicDetailLevel = "standard"
	InfoGraphicDetailLevelDetailed InfoGraphicDetailLevel = "detailed"
)

func (d InfoGraphicDetailLevel) String() string { return string(d) }
func (d InfoGraphicDetailLevel) Supported() bool {
	switch d {
	case InfoGraphicDetailLevelConcise,
		InfoGraphicDetailLevelStandard,
		InfoGraphicDetailLevelDetailed:
		return true
	}
	return false
}

func InfoGraphicDetailLevelDefault() InfoGraphicDetailLevel {
	return InfoGraphicDetailLevelStandard
}
