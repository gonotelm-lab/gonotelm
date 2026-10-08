package videooverview

import (
	"context"
	"embed"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

//go:embed skills
var videoSkillsFS embed.FS

// 单镜 subagent 的通用知识：固定技能逐文件嵌入（路径写错即编译错误），
// 主题随 visual_style 变，整个目录嵌入。
//
//go:embed skills/hyperframes-contract/SKILL.md
var shotContractSkill string

//go:embed skills/visual-style/SKILL.md
var shotVisualStyleSkill string

//go:embed skills/animation/SKILL.md
var shotAnimationSkill string

//go:embed skills/theme
var shotThemeFS embed.FS

// syncSkillsToSandbox 把内置 Agent Skills 按目录结构写入沙箱工作目录（幂等覆盖）。
func syncSkillsToSandbox(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string) error {
	return types.SyncEmbeddedTree(ctx, sandbox, workspaceDir, videoSkillsFS, "skills")
}
