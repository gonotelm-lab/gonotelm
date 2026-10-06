package stylepreview

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"mime"
	"os"
	"path/filepath"

	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/task/idregistry"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkginitjob "github.com/gonotelm-lab/gonotelm/pkg/initjob"
)

// assetSlidesPreviewDir is where the preview assets live under the configured
// assets directory. It reuses the key layout constants so the directory the
// task reads from and the key it writes to cannot drift apart.
const assetSlidesPreviewDir = previewKeyPrefix + "/" + previewKeySlides

// previewAsset is one shipped preview asset: the file inside the assets
// directory and the identifier its row is stored under.
type previewAsset struct {
	fileName   string
	identifier entity.StylePreviewIdentifier
}

// slidesPreviewAssets is the ordered manifest of the shipped preview assets.
// It is a slice rather than a map so every run walks the assets in the same
// order, which keeps partial progress and logs reproducible.
var slidesPreviewAssets = []previewAsset{
	{fileName: "cute-v1.webp", identifier: entity.StylePreviewSlidesCute},
	{fileName: "default-v1.webp", identifier: entity.StylePreviewSlidesDefault},
	{fileName: "educational-v1.webp", identifier: entity.StylePreviewSlidesEducational},
}

// slidesInitTask seeds the artifact style preview assets: it uploads each
// shipped file to the object store and upserts the row that lets an identifier
// be resolved to an image later.
type slidesInitTask struct {
	pkginitjob.Base

	stylePreviewRepo artifactrepo.StylePreviewRepository
	objectStore      adapter.ObjectStore
	keyFactory       adapter.StoreKeyFactory
	assetsDir        string
}

func NewSlidesInitTask(
	stylePreviewRepo artifactrepo.StylePreviewRepository,
	objectStore adapter.ObjectStore,
	keyFactory adapter.StoreKeyFactory,
	assetsDir string,
) pkginitjob.Task {
	return &slidesInitTask{
		stylePreviewRepo: stylePreviewRepo,
		objectStore:      objectStore,
		keyFactory:       keyFactory,
		assetsDir:        assetsDir,
	}
}

var _ pkginitjob.Task = &slidesInitTask{}

func (t *slidesInitTask) ID() string {
	return idregistry.IdStylePreviewInit
}

func (t *slidesInitTask) Description() string {
	return "Initialize artifact slides style preview assets (version: " + slidesPreviewVersionV1 + ")"
}

func (t *slidesInitTask) Run(ctx context.Context) error {
	dir := filepath.Join(t.assetsDir, assetSlidesPreviewDir)

	for _, asset := range slidesPreviewAssets {
		if err := t.initSlidesPreview(ctx, asset, filepath.Join(dir, asset.fileName)); err != nil {
			slog.ErrorContext(ctx, "initjob slides style preview failed",
				slog.String("identifier", asset.identifier.String()),
				slog.String("file_name", asset.fileName),
				slog.Any("err", err),
			)

			return pkgerrors.WithMessagef(err, "failed to init slides style preview, file_name=%s", asset.fileName)
		}
	}

	return nil
}

// initSlidesPreview makes the stored preview match the asset file. The file is
// the source of truth, so the row is never trusted on its own: a row whose
// store key is invalid, whose object has gone missing, or whose checksum no
// longer matches the file is rewritten instead of reported as an error.
func (t *slidesInitTask) initSlidesPreview(ctx context.Context, asset previewAsset, fullPath string) error {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return pkgerrors.WithMessagef(err, "failed to read asset file, file_path=%s", fullPath)
	}

	exist, err := t.stylePreviewRepo.FindByIdentifier(ctx, asset.identifier)
	switch {
	case err == nil:
		if t.previewIsUpToDate(ctx, asset, exist) {
			return nil
		}
	case pkgerrors.Is(err, artifacterrors.ErrStylePreviewNotFound):
		exist = nil
	default:
		return pkgerrors.WithMessagef(err, "failed to find style preview, identifier=%s", asset.identifier)
	}

	if err := t.storeSlidesPreview(ctx, asset, content, exist); err != nil {
		return pkgerrors.WithMessagef(err, "failed to store style preview, identifier=%s", asset.identifier)
	}

	return nil
}

// previewIsUpToDate reports whether the seed this task laid down is still
// intact: the row's store key is usable and the object it points at still
// hashes to the recorded checksum.
//
// The comparison is deliberately row-versus-bucket only. A shipped asset whose
// content changes is an update, and an update belongs to a new task, because
// this task only ever runs once per environment: comparing the asset file here
// would let a re-run of this older task undo an asset that a newer task has
// already replaced. Anything unexpected (unusable store key, missing or
// unreadable object, checksum mismatch) counts as "not up to date", because
// re-uploading is the safe side.
func (t *slidesInitTask) previewIsUpToDate(
	ctx context.Context, asset previewAsset, exist *entity.StylePreview,
) bool {
	if !exist.StoreKey.Valid() {
		return false
	}

	storedContent, _, err := t.objectStore.GetObject(ctx, exist.StoreKey)
	if err != nil {
		slog.WarnContext(ctx, "initjob slides style preview stored object is unavailable, re-uploading",
			slog.String("identifier", asset.identifier.String()),
			slog.String("store_key", exist.StoreKey.String()),
			slog.Any("err", err),
		)
		return false
	}

	if storedSum := md5Sum(storedContent); !bytes.Equal(storedSum, exist.Md5sum) {
		slog.WarnContext(ctx, "initjob slides style preview stored object does not match its recorded checksum, re-uploading",
			slog.String("identifier", asset.identifier.String()),
			slog.String("store_key", exist.StoreKey.String()),
			slog.String("recorded_md5", hex.EncodeToString(exist.Md5sum)),
			slog.String("stored_md5", hex.EncodeToString(storedSum)),
		)
		return false
	}

	return true
}

// storeSlidesPreview uploads the asset, upserts its row and then drops the
// object that row supersedes. The upload happens before the upsert, so dying
// in between can only leave an unreferenced object behind, never a row
// pointing at a missing one.
func (t *slidesInitTask) storeSlidesPreview(
	ctx context.Context, asset previewAsset, content []byte, superseded *entity.StylePreview,
) error {
	ext := filepath.Ext(asset.fileName)

	storeKey, err := t.keyFactory.New(fmtSlidesPreviewKey(slidesPreviewVersionV1, ext), true)
	if err != nil {
		return pkgerrors.WithMessagef(err, "failed to create store key, file_name=%s", asset.fileName)
	}

	if err := t.objectStore.Upload(ctx, storeKey, content, mime.TypeByExtension(ext)); err != nil {
		return pkgerrors.WithMessagef(err, "failed to upload asset, store_key=%s", storeKey)
	}

	preview := entity.NewStylePreview(asset.identifier, storeKey)
	preview.UpdateAsset(storeKey, md5Sum(content), asset.fileName)
	if err := t.stylePreviewRepo.Save(ctx, preview); err != nil {
		// Without the row the freshly uploaded object is unreachable, so drop
		// it best-effort.
		if delErr := t.objectStore.DeleteObject(ctx, storeKey); delErr != nil {
			slog.WarnContext(ctx, "initjob slides style preview failed to clean up uploaded object",
				slog.String("store_key", storeKey.String()),
				slog.Any("err", delErr),
			)
		}
		return pkgerrors.WithMessagef(err, "failed to save style preview, identifier=%s", asset.identifier)
	}

	// Best-effort: a leftover object only costs storage, so a failure to remove
	// it must not fail the task.
	if superseded != nil && superseded.StoreKey.Valid() && superseded.StoreKey != storeKey {
		if err := t.objectStore.DeleteObject(ctx, superseded.StoreKey); err != nil {
			slog.WarnContext(ctx, "initjob slides style preview failed to delete superseded object",
				slog.String("identifier", asset.identifier.String()),
				slog.String("store_key", superseded.StoreKey.String()),
				slog.Any("err", err),
			)
		}
	}

	return nil
}

func md5Sum(content []byte) []byte {
	sum := md5.Sum(content)
	return sum[:]
}
