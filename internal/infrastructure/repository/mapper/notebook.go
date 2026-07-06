package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookdomain "github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func NotebookToSchema(notebook *notebookdomain.Notebook) *schema.Notebook {
	return &schema.Notebook{
		Id:          notebook.Id,
		Name:        notebook.Name,
		Description: notebook.Description,
		OwnerId:     notebook.OwnerId,
		UpdatedAt:   notebook.UpdateTime.Value(),
	}
}

func NotebookFromSchema(notebook *schema.Notebook) *notebookdomain.Notebook {
	return &notebookdomain.Notebook{
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

func NotebooksFromSchemas(notebooks []*schema.Notebook) []*notebookdomain.Notebook {
	results := make([]*notebookdomain.Notebook, 0, len(notebooks))
	for i := range notebooks {
		results = append(results, NotebookFromSchema(notebooks[i]))
	}
	return results
}
