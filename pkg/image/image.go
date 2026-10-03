package image

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported image format")
	ErrTooLarge          = errors.New("image exceeds size limit")
	ErrInvalidImage      = errors.New("invalid image")
)

const (
	MimeTypeJPEG = "image/jpeg"
	MimeTypePNG  = "image/png"
)

var (
	pngMagic  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	jpegMagic = []byte{0xff, 0xd8, 0xff}
)

// Limits 校验图片时施加的限制,零值表示不限制该维度。
type Limits struct {
	MaxBytes     int64
	MaxDimension int
}

// Validate 校验图片并返回其真实 MIME 类型,以魔数为准而非客户端声明的 Content-Type。
func Validate(content []byte, limits Limits) (string, error) {
	if limits.MaxBytes > 0 && int64(len(content)) > limits.MaxBytes {
		return "", ErrTooLarge
	}
	if len(content) == 0 {
		return "", ErrInvalidImage
	}

	contentType, ok := SniffContentType(content)
	if !ok {
		return "", ErrUnsupportedFormat
	}

	// 声明合法但数据是解压炸弹的畸形图片只有真正解码才能证伪
	cfg, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return "", ErrInvalidImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return "", ErrInvalidImage
	}
	if limits.MaxDimension > 0 && (cfg.Width > limits.MaxDimension || cfg.Height > limits.MaxDimension) {
		return "", ErrTooLarge
	}

	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return "", ErrInvalidImage
	}
	if bounds := img.Bounds(); bounds.Dx() != cfg.Width || bounds.Dy() != cfg.Height {
		return "", ErrInvalidImage
	}

	return contentType, nil
}

// SniffContentType 按魔数判断图片类型,目前只识别 PNG 与 JPEG。
func SniffContentType(content []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(content, pngMagic):
		return MimeTypePNG, true
	case bytes.HasPrefix(content, jpegMagic):
		return MimeTypeJPEG, true
	default:
		return "", false
	}
}
