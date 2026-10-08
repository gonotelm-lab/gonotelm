package videooverview

import (
	"context"
	"embed"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

//go:embed components
var videoComponentsFS embed.FS

func syncComponentsToSandbox(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string) error {
	return types.SyncEmbeddedTree(ctx, sandbox, workspaceDir, videoComponentsFS, "components")
}
