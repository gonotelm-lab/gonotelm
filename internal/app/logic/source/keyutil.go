package source

import (
	"fmt"
	"path/filepath"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

// Format:
// file/{{notebook_id}}/{{source_id}}{{.format}}
func formatSourceStoreKey(
	params *UploadSourceParams,
	source *model.Source,
) string {
	var (
		notebookId = source.NotebookId.String()
		sourceId   = source.Id.String()
		// take extension from input filename
		ext = filepath.Ext(params.Filename)
	)

	return fmt.Sprintf("file/%s/%s%s", notebookId, sourceId, ext)
}

// Format:
func formatSourceCreateCacheKey(notebookId uuid.UUID) string {
	return fmt.Sprintf("gonotelm:source:create:lock:%s", notebookId.String())
}
