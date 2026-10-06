package slides

import (
	"context"
	"embed"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

//go:embed skills
var slidesSkillsFS embed.FS

// syncSkillsToSandbox 把内置 Agent Skills（pptxgenjs / slides-layout / slides-design / slides-qa）
// 与主题 token（skills/theme/<style>/theme.json）按目录结构写入沙箱工作目录（幂等覆盖），
// 供 agent 按需 ReadFile 加载、compile.js 直接 require。
func syncSkillsToSandbox(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string) error {
	return types.SyncEmbeddedTree(ctx, sandbox, workspaceDir, slidesSkillsFS, "skills")
}
