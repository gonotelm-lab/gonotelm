package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func NotebookToSchema(notebook *notebookentity.Notebook) *schema.Notebook {
	return &schema.Notebook{
		Id:          notebook.Id,
		Name:        notebook.Name,
		Description: notebook.Description,
		OwnerId:     notebook.OwnerId,
		UpdatedAt:   notebook.UpdateTime.Value(),
	}
}

func NotebookFromSchema(notebook *schema.Notebook) *notebookentity.Notebook {
	return &notebookentity.Notebook{
		Base: entity.Base{
			Id:         notebook.Id,
			CreateTime: valobj.NewTimeFromId(notebook.Id),
			UpdateTime: valobj.NewTimeFrom(notebook.UpdatedAt),
		},
		Name:        notebook.Name,
		Description: notebook.Description,
		OwnerId:     notebook.OwnerId,
	}
}

func NotebooksFromSchemas(notebooks []*schema.Notebook) []*notebookentity.Notebook {
	results := make([]*notebookentity.Notebook, 0, len(notebooks))
	for i := range notebooks {
		results = append(results, NotebookFromSchema(notebooks[i]))
	}
	return results
}
