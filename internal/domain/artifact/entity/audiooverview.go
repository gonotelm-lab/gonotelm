package entity

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

type ArtifactAudioOverviewStyle string

const (
	ArtifactAudioOverviewStyleDeepResearch ArtifactAudioOverviewStyle = "deep-research"
	ArtifactAudioOverviewStyleAbstract     ArtifactAudioOverviewStyle = "abstract"
	ArtifactAudioOverviewStyleDiscussion   ArtifactAudioOverviewStyle = "discussion"
	ArtifactAudioOverviewStyleDebate       ArtifactAudioOverviewStyle = "debate"
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

func ArtifactAudioOverviewStyleDefault() ArtifactAudioOverviewStyle {
	return ArtifactAudioOverviewStyleAbstract
}

type AudioSpeaker struct {
	Name        string `yaml:"name"`
	Personality string `yaml:"personality"`
	Bio         string `yaml:"bio"`
}

type AudioEpisode struct {
	Style        ArtifactAudioOverviewStyle `yaml:"style"`
	Title        string                     `yaml:"title"`
	Description  string                     `yaml:"description"`
	SpeakerKeys  []string                   `yaml:"speakers"`
	SpeakerRoles map[string]string          `yaml:"speaker_roles"`
	NumSegments  int                        `yaml:"num_of_segments"`
	SegmentFlow  []string                   `yaml:"segment_flow"`
	Speakers     []AudioSpeaker             `yaml:"-"`
}

//go:embed assets/audiospeakers.yml
var speakersYAML []byte

//go:embed assets/audioepisodes.yml
var episodesYAML []byte

var BuiltinSpeakers map[string]AudioSpeaker
var BuiltinEpisodes map[ArtifactAudioOverviewStyle]*AudioEpisode

func init() {
	var sf struct {
		Speakers map[string]AudioSpeaker `yaml:"speakers"`
	}
	if err := yaml.Unmarshal(speakersYAML, &sf); err != nil {
		panic("failed to parse audiospeakers.yml: " + err.Error())
	}
	BuiltinSpeakers = sf.Speakers

	var ef struct {
		Episodes map[string]struct {
			Style        ArtifactAudioOverviewStyle `yaml:"style"`
			Title        string                     `yaml:"title"`
			Description  string                     `yaml:"description"`
			SpeakerKeys  []string                   `yaml:"speakers"`
			SpeakerRoles map[string]string          `yaml:"speaker_roles"`
			NumSegments  int                        `yaml:"num_of_segments"`
			SegmentFlow  []string                   `yaml:"segment_flow"`
		} `yaml:"episodes"`
	}
	if err := yaml.Unmarshal(episodesYAML, &ef); err != nil {
		panic("failed to parse audioepisodes.yml: " + err.Error())
	}

	BuiltinEpisodes = make(map[ArtifactAudioOverviewStyle]*AudioEpisode, len(ef.Episodes))
	for _, ep := range ef.Episodes {
		speakers := make([]AudioSpeaker, 0, len(ep.SpeakerKeys))
		for _, key := range ep.SpeakerKeys {
			if sp, ok := BuiltinSpeakers[key]; ok {
				speakers = append(speakers, sp)
			}
		}
		BuiltinEpisodes[ep.Style] = &AudioEpisode{
			Style:        ep.Style,
			Title:        ep.Title,
			Description:  ep.Description,
			SpeakerKeys:  ep.SpeakerKeys,
			SpeakerRoles: ep.SpeakerRoles,
			NumSegments:  ep.NumSegments,
			SegmentFlow:  ep.SegmentFlow,
			Speakers:     speakers,
		}
	}
}
