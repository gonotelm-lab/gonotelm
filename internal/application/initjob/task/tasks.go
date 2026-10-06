// Package tasks holds the initialization tasks and the list that registers
// them. Adding a task is one task file plus one line in All.
package task

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/dep"
	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/task/artifact/stylepreview"
	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/task/idregistry"
	pkginitjob "github.com/gonotelm-lab/gonotelm/pkg/initjob"
)

type Option struct {
	Infra      *dep.Infra
	Dependency *dep.Dependency

	AssetsDir string
}

// New returns every registered task in any order: the runner sorts them by ID,
// so registration order never decides execution order.
func New(opt *Option) []pkginitjob.Task {
	return []pkginitjob.Task{
		stylepreview.NewTask(stylepreview.Option{
			ID:          idregistry.IdSlidesStylePreviewInit,
			Description: "Initialize artifact slides style preview assets",
			Provider:    stylepreview.SlidesProvider(),
			Repo:        opt.Dependency.StylePreviewRepo,
			ObjectStore: opt.Infra.ObjectStore,
			KeyFactory:  opt.Infra.KeyFactory,
			AssetsDir:   opt.AssetsDir,
		}),
		stylepreview.NewTask(stylepreview.Option{
			ID:          idregistry.IdInfoGraphicStylePreviewInit,
			Description: "Initialize artifact infographic style preview assets",
			Provider:    stylepreview.InfoGraphicProvider(),
			Repo:        opt.Dependency.StylePreviewRepo,
			ObjectStore: opt.Infra.ObjectStore,
			KeyFactory:  opt.Infra.KeyFactory,
			AssetsDir:   opt.AssetsDir,
		}),
	}
}
