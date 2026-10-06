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

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkginitjob "github.com/gonotelm-lab/gonotelm/pkg/initjob"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Option struct {
	ID          string
	Description string
	Provider    Provider

	Repo        artifactrepo.StylePreviewRepository
	ObjectStore adapter.ObjectStore
	KeyFactory  adapter.StoreKeyFactory
	AssetsDir   string
}

type stylePreviewTask struct {
	pkginitjob.Base

	opt Option
}

func NewTask(opt Option) pkginitjob.Task {
	return &stylePreviewTask{opt: opt}
}

var _ pkginitjob.Task = &stylePreviewTask{}

func (t *stylePreviewTask) ID() string { return t.opt.ID }

func (t *stylePreviewTask) Description() string { return t.opt.Description }

func (t *stylePreviewTask) Run(ctx context.Context) error {
	dir := filepath.Join(t.opt.AssetsDir, t.opt.Provider.LocalDir())

	for _, asset := range t.opt.Provider.Assets() {
		if err := t.initPreview(ctx, asset, filepath.Join(dir, asset.FileName)); err != nil {
			slog.ErrorContext(ctx, "initjob style preview failed",
				slog.String("kind", t.opt.Provider.Kind().String()),
				slog.String("identifier", asset.Identifier.String()),
				slog.String("file_name", asset.FileName),
				slog.Any("err", err),
			)

			return pkgerrors.WithMessagef(err, "failed to init style preview, file_name=%s", asset.FileName)
		}
	}

	return nil
}

// 以本地文件为准：记录里的 key 失效、对象丢失、校验和不符都覆盖重建，不报错。
func (t *stylePreviewTask) initPreview(ctx context.Context, asset Asset, fullPath string) error {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return pkgerrors.WithMessagef(err, "failed to read asset file, file_path=%s", fullPath)
	}

	exist, err := t.opt.Repo.FindByIdentifier(ctx, asset.Identifier)
	switch {
	case err == nil:
		if t.previewIsUpToDate(ctx, asset, exist) {
			return nil
		}
	case pkgerrors.Is(err, artifacterrors.ErrStylePreviewNotFound):
		exist = nil
	default:
		return pkgerrors.WithMessagef(err, "failed to find style preview, identifier=%s", asset.Identifier)
	}

	if err := t.storePreview(ctx, asset, content, exist); err != nil {
		return pkgerrors.WithMessagef(err, "failed to store style preview, identifier=%s", asset.Identifier)
	}

	return nil
}

// 只比对记录与 bucket：资产内容变化属于"更新"，由新 task 负责；
// 旧 task 重跑时若也看本地文件，会把新 task 换掉的图覆盖回去。
func (t *stylePreviewTask) previewIsUpToDate(
	ctx context.Context, asset Asset, exist *entity.StylePreview,
) bool {
	if !exist.StoreKey.Valid() {
		return false
	}

	storedContent, _, err := t.opt.ObjectStore.GetObject(ctx, exist.StoreKey)
	if err != nil {
		slog.WarnContext(ctx, "initjob style preview stored object is unavailable, re-uploading",
			slog.String("identifier", asset.Identifier.String()),
			slog.String("store_key", exist.StoreKey.String()),
			slog.Any("err", err),
		)
		return false
	}

	if storedSum := md5Sum(storedContent); !bytes.Equal(storedSum, exist.Md5sum) {
		slog.WarnContext(ctx, "initjob style preview stored object does not match its recorded checksum, re-uploading",
			slog.String("identifier", asset.Identifier.String()),
			slog.String("store_key", exist.StoreKey.String()),
			slog.String("recorded_md5", hex.EncodeToString(exist.Md5sum)),
			slog.String("stored_md5", hex.EncodeToString(storedSum)),
		)
		return false
	}

	return true
}

// 先上传再落库，中途失败最多留个孤儿对象，不会留下指向空对象的记录。
func (t *stylePreviewTask) storePreview(
	ctx context.Context, asset Asset, content []byte, superseded *entity.StylePreview,
) error {
	storeKey, err := t.opt.KeyFactory.New(fmtPreviewKey(t.opt.Provider.KeyPrefix(), asset.FileName), true)
	if err != nil {
		return pkgerrors.WithMessagef(err, "failed to create store key, file_name=%s", asset.FileName)
	}

	if err := t.opt.ObjectStore.Upload(
		ctx, storeKey, content, mime.TypeByExtension(filepath.Ext(asset.FileName)),
	); err != nil {
		return pkgerrors.WithMessagef(err, "failed to upload asset, store_key=%s", storeKey)
	}

	preview := entity.NewStylePreview(asset.Identifier, storeKey)
	preview.UpdateAsset(storeKey, md5Sum(content), asset.FileName)
	if err := t.opt.Repo.Save(ctx, preview); err != nil {
		if delErr := t.opt.ObjectStore.DeleteObject(ctx, storeKey); delErr != nil {
			slog.WarnContext(ctx, "initjob style preview failed to clean up uploaded object",
				slog.String("store_key", storeKey.String()),
				slog.Any("err", delErr),
			)
		}
		return pkgerrors.WithMessagef(err, "failed to save style preview, identifier=%s", asset.Identifier)
	}

	if superseded != nil && superseded.StoreKey.Valid() && superseded.StoreKey != storeKey {
		if err := t.opt.ObjectStore.DeleteObject(ctx, superseded.StoreKey); err != nil {
			slog.WarnContext(ctx, "initjob style preview failed to delete superseded object",
				slog.String("identifier", asset.Identifier.String()),
				slog.String("store_key", superseded.StoreKey.String()),
				slog.Any("err", err),
			)
		}
	}

	return nil
}

// 对象 key：<keyPrefix>/<uuid><ext>，keyPrefix 由 Provider 显式给出。
func fmtPreviewKey(keyPrefix, fileName string) string {
	key := keyPrefix + "/" + uuid.NewV4().String()
	if ext := filepath.Ext(fileName); ext != "" {
		key += ext
	}

	return key
}

func md5Sum(content []byte) []byte {
	sum := md5.Sum(content)
	return sum[:]
}
