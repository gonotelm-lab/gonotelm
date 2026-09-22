package videooverview

import (
	"context"
	"embed"

	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

//go:embed components
var videoComponentsFS embed.FS

func syncComponentsToSandbox(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string) error {
	return syncEmbeddedTree(ctx, sandbox, workspaceDir, videoComponentsFS, "components")
}
