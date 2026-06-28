package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domain "github.com/gonotelm-lab/gonotelm/internal/domain/source"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
)

func SourceToSchema(source *domain.Source) *schema.Source {
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

func SourceFromSchema(source *schema.Source) (*domain.Source, error) {
	domainSource := &domain.Source{
		Base: entity.Base{
			Id:         source.Id,
			CreateTime: valobj.NewTimeFromId(source.Id),
			UpdateTime: valobj.NewTimeFromId(source.Id),
		},
		NotebookId: source.NotebookId,
		Kind:       domain.SourceKind(source.Kind),
		Status:     domain.SourceStatus(source.Status),
		Title:      source.Title,
		// Content: source.Content, // TODO
		ParsedContentKey: source.ParsedContentKey,
		Abstract:         source.Abstract,
		OwnerId:          source.OwnerId,
	}
	content, err := domain.NewSourceContent(domain.SourceKind(source.Kind), source.Content)
	if err != nil {
		return nil, err
	}
	domainSource.Content = content

	return domainSource, nil
}

func SourcesFromSchemas(sources []*schema.Source) ([]*domain.Source, error) {
	results := make([]*domain.Source, 0, len(sources))
	for i := range sources {
		source, err := SourceFromSchema(sources[i])
		if err != nil {
			return nil, err
		}
		results = append(results, source)
	}

	return results, nil
}
