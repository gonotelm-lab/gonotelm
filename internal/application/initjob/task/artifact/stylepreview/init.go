package stylepreview

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/task/idregistry"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	pkginitjob "github.com/gonotelm-lab/gonotelm/pkg/initjob"
)

func NewSlidesStylePreviewInitTask(
	repo artifactrepo.StylePreviewRepository,
	objectStore adapter.ObjectStore,
	keyFactory adapter.StoreKeyFactory,
	assetsDir string,
) pkginitjob.Task {
	return newTask(option{
		ID:          idregistry.IdSlidesStylePreviewInit,
		Description: "Initialize artifact slides style preview assets",
		Provider:    SlidesProvider(),
		Repo:        repo,
		ObjectStore: objectStore,
		KeyFactory:  keyFactory,
		AssetsDir:   assetsDir,
	})
}

func NewInfoGraphicStylePreviewInitTask(
	repo artifactrepo.StylePreviewRepository,
	objectStore adapter.ObjectStore,
	keyFactory adapter.StoreKeyFactory,
	assetsDir string,
) pkginitjob.Task {
	return newTask(option{
		ID:          idregistry.IdInfoGraphicStylePreviewInit,
		Description: "Initialize artifact infographic style preview assets",
		Provider:    InfoGraphicProvider(),
		Repo:        repo,
		ObjectStore: objectStore,
		KeyFactory:  keyFactory,
		AssetsDir:   assetsDir,
	})
}

func NewVideoOverviewStylePreviewInitTask(
	repo artifactrepo.StylePreviewRepository,
	objectStore adapter.ObjectStore,
	keyFactory adapter.StoreKeyFactory,
	assetsDir string,
) pkginitjob.Task {
	return newTask(option{
		ID:          idregistry.IdVideoOverviewStylePreviewInit,
		Description: "Initialize artifact video overview style preview assets",
		Provider:    VideoOverviewProvider(),
		Repo:        repo,
		ObjectStore: objectStore,
		KeyFactory:  keyFactory,
		AssetsDir:   assetsDir,
	})
}
