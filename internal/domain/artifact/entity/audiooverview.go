package entity

type ArtifactAudioOverviewStyle string

const (
	ArtifactAudioOverviewStyleDeepResearch ArtifactAudioOverviewStyle = "deep-research"
	ArtifactAudioOverviewStyleAbstract    ArtifactAudioOverviewStyle = "abstract"
	ArtifactAudioOverviewStyleDiscussion  ArtifactAudioOverviewStyle = "discussion"
	ArtifactAudioOverviewStyleDebate      ArtifactAudioOverviewStyle = "debate"
)

func (s ArtifactAudioOverviewStyle) String() string { return string(s) }
func (s ArtifactAudioOverviewStyle) Supported() bool {
	switch s {
	case ArtifactAudioOverviewStyleDeepResearch,
		ArtifactAudioOverviewStyleAbstract,
		ArtifactAudioOverviewStyleDiscussion,
		ArtifactAudioOverviewStyleDebate:
		return true
	}
	return false
}