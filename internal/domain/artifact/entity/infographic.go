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
