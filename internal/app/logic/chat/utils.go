package chat

import (
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
)

type groupedDocs struct {
	SourceId string
	DocIds   []string
}

func buildGroupedDocs(sourceDocs []*model.SourceDoc) []*groupedDocs {
	if len(sourceDocs) == 0 {
		return nil
	}

	groups := make([]*groupedDocs, 0, len(sourceDocs))
	for _, sourceDoc := range sourceDocs {
		if sourceDoc == nil {
			continue
		}

		sourceID := sourceDoc.SourceId.String()
		groupIdx := -1
		for idx, group := range groups {
			if group.SourceId == sourceID {
				groupIdx = idx
				break
			}
		}
		if groupIdx < 0 {
			groups = append(groups, &groupedDocs{
				SourceId: sourceID,
				DocIds:   make([]string, 0, 1),
			})
			groupIdx = len(groups) - 1
		}

		if sourceDoc.Id != "" {
			groups[groupIdx].DocIds = append(groups[groupIdx].DocIds, sourceDoc.Id)
		}
	}

	return groups
}

func buildMessageExtra(sourceDocs []*model.SourceDoc) *chatmodel.MessageExtra {
	groupedSourceDocs := buildGroupedDocs(sourceDocs)
	if len(groupedSourceDocs) == 0 {
		return nil
	}

	citations := make([]*chatmodel.Citation, 0, len(groupedSourceDocs))
	for _, grouped := range groupedSourceDocs {
		citations = append(citations, &chatmodel.Citation{
			SourceId: grouped.SourceId,
			DocIds:   grouped.DocIds,
		})
	}

	return &chatmodel.MessageExtra{
		Citation: citations,
	}
}

func buildPhaseCitation(sourceDocs []*model.SourceDoc) []*chatmodel.PhaseCitationItem {
	groupedSourceDocs := buildGroupedDocs(sourceDocs)
	if len(groupedSourceDocs) == 0 {
		return nil
	}

	docsMap := slices.AsMapF(sourceDocs,
		func(doc *model.SourceDoc) string { return doc.Id },
	)

	items := make([]*chatmodel.PhaseCitationItem, 0, len(groupedSourceDocs))
	for _, grouped := range groupedSourceDocs {
		item := &chatmodel.PhaseCitationItem{
			SourceId: grouped.SourceId,
			Docs:     make([]*chatmodel.PhaseCitationDoc, 0, len(grouped.DocIds)),
		}
		for _, docID := range grouped.DocIds {
			start, end := 0, 0
			isSummary := true
			doc, ok := docsMap[docID]
			if ok {
				start = doc.RunePos.GetStart()
				end = doc.RunePos.GetEnd()
				isSummary = doc.IsDerived()
			}

			item.Docs = append(item.Docs, &chatmodel.PhaseCitationDoc{
				Id:        docID,
				IsSummary: isSummary,
				Position: &chatmodel.PhaseCitationDocPosition{
					Start: start,
					End:   end,
				},
			})
		}
		items = append(items, item)
	}

	return items
}

func buildChatTemplateVars(state *sessionState) prompts.ChatTemplateVars {
	sourceDocs := state.sourceDocs
	templateVars := prompts.ChatTemplateVars{}

	for _, sourceDoc := range sourceDocs {
		if sourceDoc == nil {
			continue
		}

		sourceID := sourceDoc.SourceId.String()
		groupIdx := -1
		for idx, group := range templateVars.SelectedSources {
			if group.SourceID == sourceID {
				groupIdx = idx
				break
			}
		}
		if groupIdx < 0 {
			templateVars.SelectedSources = append(templateVars.SelectedSources,
				prompts.ChatSelectedSourceGroup{
					SourceIndex: int64(len(templateVars.SelectedSources)),
					SourceID:    sourceID,
				})
			groupIdx = len(templateVars.SelectedSources) - 1
		}
		docIndex := int64(len(templateVars.SelectedSources[groupIdx].Docs))

		templateVars.SelectedSources[groupIdx].Docs = append(
			templateVars.SelectedSources[groupIdx].Docs,
			prompts.ChatSelectedSourceDoc{
				DocIndex: docIndex,
				DocID:    sourceDoc.Id,
				Content:  sourceDoc.Content,
				Score:    sourceDoc.Score,
			},
		)
	}

	templateVars.Style = state.chatStyle
	templateVars.AnswerLength = state.chatAnswerLength

	return templateVars
}
