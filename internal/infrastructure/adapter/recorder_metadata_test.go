package adapter

import (
	"context"
	"testing"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWithNotebookMetadata(t *testing.T) {
	notebookId := uuid.NewV7()
	ctx := pkgcontext.WithNotebookId(context.Background(), notebookId)

	metadata := withNotebookMetadata(ctx, nil)
	assert.Equal(t, notebookId.String(), metadata[notebookIdMetadataKey])

	metadata = withNotebookMetadata(context.Background(), nil)
	assert.Nil(t, metadata)

	metadata = withNotebookMetadata(context.Background(), map[string]string{"k": "v"})
	assert.Equal(t, "v", metadata["k"])
	assert.NotContains(t, metadata, notebookIdMetadataKey)
}
