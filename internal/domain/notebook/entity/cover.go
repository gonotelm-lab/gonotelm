package entity

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	coreadapter "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	pkguuid "github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/gabriel-vasile/mimetype"
)

const (
	CoverObjectPathPrefix = "notebooks/cover/"
	MaxCoverBytes         = 2 << 20
	MaxCoverDimension     = 2048
	CoverHeadProbeBytes   = 3072           // 与 mimetype 默认探测长度一致
	CoverCleanupDelay     = 24 * time.Hour // 检查点 fireAt = 链接生成 + 1 天
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

// GenerateCoverKey 生成一个新的封面在途 key：prefix + 随机 token，公有读。
func (n *Notebook) GenerateCoverKey(keyFactory coreadapter.StoreKeyFactory) (valobj.StoreKey, error) {
	token, err := pkguuid.NewRandomURLToken(8)
	if err != nil {
		return valobj.StoreKey{}, err
	}

	return keyFactory.New(CoverObjectPathPrefix+token, true)
}

// BeginCoverFlow 幂等下发封面在途 key
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
