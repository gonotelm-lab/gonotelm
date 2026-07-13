package audiooverview

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

//go:embed podcast-outline.jinja
var podcastOutlinePromptContent string

var podcastOutlineTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(podcastOutlinePromptContent))

//go:embed podcast-transcript.jinja
var podcastTranscriptPromptContent string

var podcastTranscriptTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(podcastTranscriptPromptContent))

func RenderPodcastOutline(
	ctx context.Context,
	sourceIds []string,
	lang string,
	tips string,
	style artifactentity.ArtifactAudioOverviewStyle,
) ([]*einoschema.Message, error) {
	ep, ok := artifactentity.BuiltinEpisodes[style]
	if !ok {
		ep = artifactentity.BuiltinEpisodes[artifactentity.ArtifactAudioOverviewStyleDefault()]
	}

	vars := StudioPodcastOutlineTemplateVars{
		SourceIds:     sourceIds,
		Language:      lang,
		Tips:          tips,
		Style:         ep.Style,
		StyleDesc:     ep.Description,
		Speakers:      ep.Speakers,
		NumOfSegments: ep.NumSegments,
	}

	msgs, err := podcastOutlineTpl.Format(ctx, vars.PromptVars())
	if err != nil {
		return nil, fmt.Errorf("render podcast outline prompt: %w", err)
	}
	return msgs, nil
}

func RenderPodcastTranscript(
	ctx context.Context,
	sourceIds []string,
	lang string,
	tips string,
	style artifactentity.ArtifactAudioOverviewStyle,
	outline *podcastOutlineExpectation,
) ([]*einoschema.Message, error) {
	ep, ok := artifactentity.BuiltinEpisodes[style]
	if !ok {
		ep = artifactentity.BuiltinEpisodes[artifactentity.ArtifactAudioOverviewStyleDefault()]
	}

	segments := make([]map[string]string, 0, len(outline.Segments))
	for _, seg := range outline.Segments {
		segments = append(segments, map[string]string{
			"name":    seg.Name,
			"content": seg.Content,
		})
	}

	vars := StudioPodcastTranscriptTemplateVars{
		SourceIds:    sourceIds,
		Language:     lang,
		Tips:         tips,
		Style:        ep.Style,
		StyleDesc:    ep.Description,
		Speakers:     ep.Speakers,
		SpeakerRoles: ep.SpeakerRoles,
		SegmentFlow:  ep.SegmentFlow,
		Outline: map[string]any{
			"title":    outline.Title,
			"segments": segments,
		},
	}

	msgs, err := podcastTranscriptTpl.Format(ctx, vars.PromptVars())
	if err != nil {
		return nil, fmt.Errorf("render podcast transcript prompt: %w", err)
	}
	return msgs, nil
}

type StudioPodcastOutlineTemplateVars struct {
	SourceIds     []string
	Speakers      []artifactentity.AudioSpeaker
	Tips          string
	NumOfSegments int
	Language      string
	Style         artifactentity.ArtifactAudioOverviewStyle
	StyleDesc     string
}

func (v StudioPodcastOutlineTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"SourceIds":     types.NormalizeStrings(v.SourceIds),
		"Speakers":      v.Speakers,
		"Tips":          v.Tips,
		"NumOfSegments": v.NumOfSegments,
		"Language":      v.Language,
		"StyleInfo": map[string]any{
			"Style":       v.Style,
			"Description": v.StyleDesc,
		},
	}
}

type StudioPodcastTranscriptTemplateVars struct {
	SourceIds    []string
	Speakers     []artifactentity.AudioSpeaker
	SpeakerRoles map[string]string
	SegmentFlow  []string
	Tips         string
	Language     string
	Style        artifactentity.ArtifactAudioOverviewStyle
	StyleDesc    string
	Outline      map[string]any
}

func (v StudioPodcastTranscriptTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"SourceIds":    types.NormalizeStrings(v.SourceIds),
		"Speakers":     v.Speakers,
		"SpeakerRoles": v.SpeakerRoles,
		"SegmentFlow":  v.SegmentFlow,
		"Tips":         v.Tips,
		"Language":     v.Language,
		"StyleInfo": map[string]any{
			"Style":       v.Style,
			"Description": v.StyleDesc,
		},
		"Outline": v.Outline,
	}
}
