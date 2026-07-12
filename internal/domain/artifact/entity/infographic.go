package entity

type ArtifactInfoGraphicOrientation string

const (
	ArtifactInfoGraphicOrientationPortrait  ArtifactInfoGraphicOrientation = "portrait"
	ArtifactInfoGraphicOrientationLandscape ArtifactInfoGraphicOrientation = "landscape"
	ArtifactInfoGraphicOrientationSquare    ArtifactInfoGraphicOrientation = "square"
)

func (o ArtifactInfoGraphicOrientation) String() string { return string(o) }
func (o ArtifactInfoGraphicOrientation) Supported() bool {
	switch o {
	case ArtifactInfoGraphicOrientationPortrait,
		ArtifactInfoGraphicOrientationLandscape,
		ArtifactInfoGraphicOrientationSquare:
		return true
	}
	return false
}
func (o ArtifactInfoGraphicOrientation) ImageSize() (int, int) {
	switch o {
	case ArtifactInfoGraphicOrientationPortrait:
		return 720, 1280
	case ArtifactInfoGraphicOrientationLandscape:
		return 1280, 720
	case ArtifactInfoGraphicOrientationSquare:
		return 1024, 1024
	}
	return 1280, 720
}

func ArtifactInfoGraphicOrientationDefault() ArtifactInfoGraphicOrientation {
	return ArtifactInfoGraphicOrientationLandscape
}

type ArtifactInfoGraphicDetailLevel string

const (
	ArtifactInfoGraphicDetailLevelConcise  ArtifactInfoGraphicDetailLevel = "concise"
	ArtifactInfoGraphicDetailLevelStandard ArtifactInfoGraphicDetailLevel = "standard"
	ArtifactInfoGraphicDetailLevelDetailed ArtifactInfoGraphicDetailLevel = "detailed"
)

func (d ArtifactInfoGraphicDetailLevel) String() string { return string(d) }
func (d ArtifactInfoGraphicDetailLevel) Supported() bool {
	switch d {
	case ArtifactInfoGraphicDetailLevelConcise,
		ArtifactInfoGraphicDetailLevelStandard,
		ArtifactInfoGraphicDetailLevelDetailed:
		return true
	}
	return false
}

func ArtifactInfoGraphicDetailLevelDefault() ArtifactInfoGraphicDetailLevel {
	return ArtifactInfoGraphicDetailLevelStandard
}
