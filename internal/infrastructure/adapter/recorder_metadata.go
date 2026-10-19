package adapter

import (
	"context"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

const notebookIdMetadataKey = "notebook_id"

func withNotebookMetadata(ctx context.Context, metadata map[string]string) map[string]string {
	notebookId := pkgcontext.GetNotebookId(ctx)
	if notebookId.IsZero() {
		return metadata
	}

	if metadata == nil {
		metadata = make(map[string]string, 1)
	}
	metadata[notebookIdMetadataKey] = notebookId.String()

	return metadata
}
