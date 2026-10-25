package entity

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	coreadapter "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	pkguuid "github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/gabriel-vasile/mimetype"
	_ "golang.org/x/image/webp"
)

const (
	CoverObjectPathPrefix = "notebooks/cover/"
	MaxCoverBytes         = 2 << 20 // 2MB
	MaxCoverDimension     = 2048
	CoverHeadProbeBytes   = 3072
	CoverCleanupDelay     = 24 * time.Hour
)

var coverMimeWhitelist = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// CoverContentTypeFromMagic 按白名单探测封面类型，未命中 ("", false)。
func CoverContentTypeFromMagic(head []byte) (string, bool) {
	detected := mimetype.Detect(head).String()
	if coverMimeWhitelist[detected] {
		return detected, true
	}

	return "", false
}

// ValidateCover 校验封面图片，非法返回 ErrInvalidCover，通过返回 mime。
func ValidateCover(objectSize int64, head []byte) (string, error) {
	if objectSize > MaxCoverBytes {
		return "", notebookerrors.ErrInvalidCover
	}

	mimeType, ok := CoverContentTypeFromMagic(head)
	if !ok {
		return "", notebookerrors.ErrInvalidCover
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(head))
	if err != nil {
		return "", notebookerrors.ErrInvalidCover
	}

	if cfg.Width > MaxCoverDimension || cfg.Height > MaxCoverDimension {
		return "", notebookerrors.ErrInvalidCover
	}

	return mimeType, nil
}

func CoverContentTypeValid(mime string) bool {
	return coverMimeWhitelist[mime]
}

// CheckCoverKey 非 public / 前缀不符 → ErrInvalidCover。
func CheckCoverKey(key valobj.StoreKey) error {
	if !key.Valid() || !key.IsPublic || !strings.HasPrefix(key.Key, CoverObjectPathPrefix) {
		return notebookerrors.ErrInvalidCover
	}

	return nil
}

// GenerateCoverKey 生成一个新的封面key 公有读
func (n *Notebook) GenerateCoverKey(keyFactory coreadapter.StoreKeyFactory) (valobj.StoreKey, error) {
	token, err := pkguuid.NewRandomURLToken(8)
	if err != nil {
		return valobj.StoreKey{}, err
	}

	return keyFactory.New(CoverObjectPathPrefix+token, true)
}

// BeginCoverFlow 幂等下发封面storeKey
// 如果已经存在申请过的storeKey但是没有完成上传确认 会复用这个storeKey
func (n *Notebook) BeginCoverFlow(keyFactory coreadapter.StoreKeyFactory) (valobj.StoreKey, bool, error) {
	// 已有在途cover 直接返回
	if n.PendingCover.Valid() {
		return n.PendingCover, false, nil
	}

	// 生成一个新的
	key, err := n.GenerateCoverKey(keyFactory)
	if err != nil {
		return valobj.StoreKey{}, false, err
	}

	n.PendingCover = key
	return key, true, nil
}

func (n *Notebook) SetCover(key valobj.StoreKey) error {
	if err := CheckCoverKey(key); err != nil {
		return err
	}

	n.Cover = key
	n.PendingCover = valobj.StoreKey{}
	n.UpdateTime = valobj.NewTime()
	return nil
}

func (n *Notebook) ClearCover() {
	n.Cover = valobj.StoreKey{}
	n.PendingCover = valobj.StoreKey{}
	n.UpdateTime = valobj.NewTime()
}

func (n *Notebook) ClearPendingCover() {
	n.PendingCover = valobj.StoreKey{}
}
