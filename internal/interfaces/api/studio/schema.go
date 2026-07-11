package studio

import (
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Kind = artifactentity.Kind
type Status = artifactentity.Status
type ResultKind = artifactentity.ResultKind

type GenerateArtifactRequest struct {
	NotebookId    uuid.UUID                        `json:"notebook_id,required"`
	Kind          Kind                             `json:"kind,required"`
	SourceIds     []uuid.UUID                      `json:"source_ids,required"`
	InfoGraphic   *GenerateInfoGraphicParameters   `json:"info_graphic,omitempty"`
	AudioOverview *GenerateAudioOverviewParameters `json:"audio_overview,omitempty"`
}

type GenerateArtifactResponse struct {
	TaskId string `json:"task_id"`
}

type ArtifactTaskIdRequest struct {
	TaskId uuid.UUID `path:"task_id,required"`
}

type GetArtifactStatusResponse struct {
	TaskId string                `json:"task_id"`
	Status artifactentity.Status `json:"status"`
}

type GetArtifactResultResponse struct {
	TaskId      string                    `json:"task_id"`
	Status      artifactentity.Status     `json:"status"`
	Title       string                    `json:"title"`
	Content     string                    `json:"content,omitempty"`
	ContentUrl  string                    `json:"content_url,omitempty"`
	ContentKind artifactentity.ResultKind `json:"content_kind"`
	MimeType    string                    `json:"mime_type,omitempty"`
	ImageInfo   *ArtifactResultImageInfo  `json:"image_info,omitempty"`
}

type ListNotebookArtifactsRequest struct {
	Id     uuid.UUID `path:"id,required"`
	Limit  int       `query:"limit"`
	Offset int       `query:"offset"`
}

type ListNotebookArtifactsResponse struct {
	Artifacts []*ArtifactResult `json:"artifacts"`
	Limit     int               `json:"limit"`
	Offset    int               `json:"offset"`
	HasMore   bool              `json:"has_more"`
}

type GenerateInfoGraphicParameters struct {
	ExtraPrompt  string                                        `json:"extra_prompt,omitempty"`
	TextLanguage string                                        `json:"text_language,omitempty"`
	Orientation  artifactentity.ArtifactInfoGraphicOrientation `json:"orientation,omitempty"`
	DetailLevel  artifactentity.ArtifactInfoGraphicDetailLevel `json:"detail_level,omitempty"`
}

type GenerateAudioOverviewParameters struct {
	Tip      string                                    `json:"tip,omitempty"`
	Language string                                    `json:"language,omitempty"`
	Style    artifactentity.ArtifactAudioOverviewStyle `json:"style,omitempty"`
}

type ArtifactResult struct {
	NotebookId  string                   `json:"notebook_id"`
	TaskId      string                   `json:"task_id"`
	Kind        string                   `json:"kind"`
	Status      string                   `json:"status"`
	Title       string                   `json:"title"`
	SourceIds   []uuid.UUID              `json:"source_ids,omitempty"`
	Timestamp   int64                    `json:"timestamp"`
	Content     string                   `json:"content,omitempty"`
	ContentUrl  string                   `json:"content_url,omitempty"`
	ContentKind string                   `json:"content_kind"`
	MimeType    string                   `json:"mime_type"`
	ImageInfo   *ArtifactResultImageInfo `json:"image_info,omitempty"`
	Extras      any                      `json:"extras,omitempty"`
}

type ArtifactResultImageInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type InfoGraphicExtras struct {
	Prompt       string `json:"prompt"`
	TextLanguage string `json:"text_language"`
	Orientation  string `json:"orientation"`
	DetailLevel  string `json:"detail_level"`
}

func ToArtifactResult(a *artifactentity.Artifact) *ArtifactResult {
	r := &ArtifactResult{
		NotebookId:  a.NotebookId.String(),
		TaskId:      a.Id.String(),
		Kind:        a.Kind.String(),
		Status:      a.Status.String(),
		Title:       a.Title,
		Timestamp:   a.CreateTime.Value(),
		ContentKind: a.ResultKind.String(),
	}

	switch p := a.Payload.(type) {
	case *artifactentity.MindmapPayload:
		r.SourceIds = p.SourceIds
	case *artifactentity.ReportPayload:
		r.SourceIds = p.SourceIds
	case *artifactentity.InfoGraphicPayload:
		r.SourceIds = p.SourceIds
		r.Extras = &InfoGraphicExtras{
			Prompt:       p.ExtraPrompt,
			TextLanguage: p.TextLanguage,
			Orientation:  p.Orientation.String(),
			DetailLevel:  p.DetailLevel.String(),
		}
	case *artifactentity.AudioOverviewPayload:
		r.SourceIds = p.SourceIds
	}

	if a.ResultKind.Inline() && a.Result != nil {
		r.Content = string(a.Result)
	}

	return r
}

func (r *GenerateInfoGraphicParameters) ToPayload() *artifactentity.InfoGraphicPayload {
	if r == nil {
		return nil
	}
	return &artifactentity.InfoGraphicPayload{
		ExtraPrompt:  r.ExtraPrompt,
		TextLanguage: r.TextLanguage,
		Orientation:  r.Orientation,
		DetailLevel:  r.DetailLevel,
	}
}

func (r *GenerateAudioOverviewParameters) ToPayload() *artifactentity.AudioOverviewPayload {
	if r == nil {
		return nil
	}
	return &artifactentity.AudioOverviewPayload{
		Tip:      r.Tip,
		Language: r.Language,
		Style:    r.Style,
	}
}

func ToArtifactResults(artifacts []*artifactentity.Artifact) []*ArtifactResult {
	results := make([]*ArtifactResult, 0, len(artifacts))
	for _, a := range artifacts {
		results = append(results, ToArtifactResult(a))
	}
	return results
}
