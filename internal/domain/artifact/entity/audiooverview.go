package entity

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

type AudioOverviewStyle string

const (
	AudioOverviewStyleDeepResearch AudioOverviewStyle = "deep-research"
	AudioOverviewStyleAbstract     AudioOverviewStyle = "abstract"
	AudioOverviewStyleDiscussion   AudioOverviewStyle = "discussion"
	AudioOverviewStyleDebate       AudioOverviewStyle = "debate"
)

func (s AudioOverviewStyle) String() string { return string(s) }
func (s AudioOverviewStyle) Supported() bool {
	switch s {
	case AudioOverviewStyleDeepResearch,
		AudioOverviewStyleAbstract,
		AudioOverviewStyleDiscussion,
		AudioOverviewStyleDebate:
		return true
	}
	return false
}

func AudioOverviewStyleDefault() AudioOverviewStyle {
	return AudioOverviewStyleAbstract
}

type AudioSpeaker struct {
	Key         string                       `yaml:"key"`
	Name        string                       `yaml:"name"`
	Personality string                       `yaml:"personality"`
	Bio         string                       `yaml:"bio"`
	Gender      string                       `yaml:"gender"`
	Voices      map[string]map[string]string `yaml:"voices"` // provider → (language → voice_id)
}

type AudioEpisode struct {
	Style        AudioOverviewStyle `yaml:"style"`
	Title        string             `yaml:"title"`
	Description  string             `yaml:"description"`
	SpeakerKeys  []string           `yaml:"speakers"`
	SpeakerRoles map[string]string  `yaml:"speaker_roles"`
	NumSegments  int                `yaml:"num_of_segments"`
	SegmentFlow  []string           `yaml:"segment_flow"`
	Speakers     []AudioSpeaker     `yaml:"-"`
}

//go:embed assets/audiospeakers.yml
var speakersYaml []byte

//go:embed assets/audioepisodes.yml
var episodesYaml []byte

var (
	BuiltinSpeakers map[string]AudioSpeaker
	BuiltinEpisodes map[AudioOverviewStyle]*AudioEpisode
)

func init() {
	var sf struct {
		Speakers map[string]AudioSpeaker `yaml:"speakers"`
	}
	if err := yaml.Unmarshal(speakersYaml, &sf); err != nil {
		panic("failed to parse audiospeakers.yml: " + err.Error())
	}
	BuiltinSpeakers = make(map[string]AudioSpeaker, len(sf.Speakers))
	for k, sp := range sf.Speakers {
		sp.Key = k
		BuiltinSpeakers[k] = sp
	}

	var ef struct {
		Episodes map[string]struct {
			Style        AudioOverviewStyle `yaml:"style"`
			Title        string             `yaml:"title"`
			Description  string             `yaml:"description"`
			SpeakerKeys  []string           `yaml:"speakers"`
			SpeakerRoles map[string]string  `yaml:"speaker_roles"`
			NumSegments  int                `yaml:"num_of_segments"`
			SegmentFlow  []string           `yaml:"segment_flow"`
		} `yaml:"episodes"`
	}
	if err := yaml.Unmarshal(episodesYaml, &ef); err != nil {
		panic("failed to parse audioepisodes.yml: " + err.Error())
	}

	BuiltinEpisodes = make(map[AudioOverviewStyle]*AudioEpisode, len(ef.Episodes))
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
