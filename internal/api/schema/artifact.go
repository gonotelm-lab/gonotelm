package schema

import (
	"github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type ArtifactResult struct {
	NotebookId string               `json:"notebook_id"`
	TaskId     string               `json:"task_id"`
	Kind       model.ArtifactKind   `json:"kind"`
	Status     model.ArtifactStatus `json:"status"`
	Title      string               `json:"title"`
	SourceIds  []uuid.UUID          `json:"source_ids,omitempty"`
	Timestamp  int64                `json:"timestamp"` // unix timestamp

	// content解释
	//
	// 如果contentKind=inline, 则content为inline内容
	// 如果contentKind=storage, 则contentUrl为产物的链接 需要请求这个链接才能获取产物
	Content     string                   `json:"content,omitempty"`
	ContentUrl  string                   `json:"content_url,omitempty"`
	ContentKind model.ArtifactResultKind `json:"content_kind"` // inline | storage

	// either InfoGraphicArtifactExtras
	Extras any `json:"extras,omitempty"`
}

type InfoGraphicArtifactExtras struct {
	Prompt       string `json:"prompt"`
	TextLanguage string `json:"text_language"`
	Orientation  string `json:"orientation"`
	DetailLevel  string `json:"detail_level"`
}

func ToInfoGraphicArtifactExtras(m *studio.InfoGraphicExtrasParams) *InfoGraphicArtifactExtras {
	return &InfoGraphicArtifactExtras{
		Prompt:       m.ExtraPrompt,
		TextLanguage: m.TextLanguage,
		Orientation:  m.Orientation.String(),
		DetailLevel:  m.DetailLevel.String(),
	}
}

func ToArtifactResult(artifact *studio.Artifact) *ArtifactResult {
	r := &ArtifactResult{
		NotebookId:  artifact.NotebookId.String(),
		TaskId:      artifact.Id.String(),
		Status:      artifact.Status,
		Kind:        artifact.Kind,
		Title:       artifact.Title,
		SourceIds:   artifact.SourceIds,
		Timestamp:   artifact.Timestamp,
		Content:     artifact.Content,
		ContentUrl:  artifact.ContentUrl,
		ContentKind: artifact.ResultKind,
	}

	switch artifact.Kind {
	case model.ArtifactKindInfoGraphic:
		r.Extras = ToInfoGraphicArtifactExtras(artifact.InfoGraphic)
	}

	return r
}

func ToArtifactResults(artifacts []*studio.Artifact) []*ArtifactResult {
	results := make([]*ArtifactResult, 0, len(artifacts))
	for _, artifact := range artifacts {
		results = append(results, ToArtifactResult(artifact))
	}

	return results
}
