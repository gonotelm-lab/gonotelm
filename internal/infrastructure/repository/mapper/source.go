package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
)

func SourceToSchema(source *sourceentity.Source) *schema.Source {
	return &schema.Source{
		Id:               source.Id,
		NotebookId:       source.NotebookId,
		Kind:             string(source.Kind),
		Status:           string(source.Status),
		Title:            source.Title,
		Content:          source.Content.Bytes(),
		ParsedContentKey: source.ParsedContentKey,
		Abstract:         source.Abstract,
		OwnerId:          source.OwnerId,
		UpdatedAt:        source.UpdateTime.Value(),
	}
}

func SourceFromSchema(source *schema.Source) (*sourceentity.Source, error) {
	domainSource := &sourceentity.Source{
		Base: entity.Base{
			Id:         source.Id,
			CreateTime: valobj.NewTimeFromId(source.Id),
			UpdateTime: valobj.NewTimeFromId(source.Id),
		},
		NotebookId: source.NotebookId,
		Kind:       sourcevo.SourceKind(source.Kind),
		Status:     sourcevo.SourceStatus(source.Status),
		Title:      source.Title,
		// Content: source.Content, // TODO
		ParsedContentKey: source.ParsedContentKey,
		Abstract:         source.Abstract,
		OwnerId:          source.OwnerId,
	}
	content, err := sourceentity.NewSourceContent(sourcevo.SourceKind(source.Kind), source.Content)
	if err != nil {
		return nil, err
	}
	domainSource.Content = content

	return domainSource, nil
}

func SourcesFromSchemas(sources []*schema.Source) ([]*sourceentity.Source, error) {
	results := make([]*sourceentity.Source, 0, len(sources))
	for i := range sources {
		source, err := SourceFromSchema(sources[i])
		if err != nil {
			return nil, err
		}
		results = append(results, source)
	}

	return results, nil
}
