package image

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return buf.Bytes()
}

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	return buf.Bytes()
}

func TestValidateAcceptsPNGAndJPEG(t *testing.T) {
	limits := Limits{MaxBytes: 1024 * 1024, MaxDimension: 1024}

	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"png", encodePNG(t, 64, 64), MimeTypePNG},
		{"jpeg", encodeJPEG(t, 64, 64), MimeTypeJPEG},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Validate(tc.data, limits)
			if err != nil {
				t.Fatalf("expected valid image, got %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	limits := Limits{MaxBytes: 1024 * 1024, MaxDimension: 1024}

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"not an image", []byte("hello world, definitely not an image")},
		{"png magic but truncated", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}},
		{"jpeg magic but truncated", []byte{0xff, 0xd8, 0xff, 0xe0}},
		{"gif is unsupported", []byte("GIF89a")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Validate(tc.data, limits); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateEnforcesByteLimit(t *testing.T) {
	data := encodePNG(t, 64, 64)
	limits := Limits{MaxBytes: int64(len(data) - 1), MaxDimension: 1024}

	if _, err := Validate(data, limits); err != ErrTooLarge {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestValidateEnforcesDimensionLimit(t *testing.T) {
	data := encodePNG(t, 128, 64)

	if _, err := Validate(data, Limits{MaxBytes: 1024 * 1024, MaxDimension: 64}); err != ErrTooLarge {
		t.Fatalf("expected ErrTooLarge for width, got %v", err)
	}
	if _, err := Validate(data, Limits{MaxBytes: 1024 * 1024, MaxDimension: 127}); err != ErrTooLarge {
		t.Fatalf("expected ErrTooLarge for width, got %v", err)
	}
	if _, err := Validate(data, Limits{MaxBytes: 1024 * 1024, MaxDimension: 128}); err != nil {
		t.Fatalf("expected width at the limit to pass, got %v", err)
	}
}

func TestValidateRejectsDimensionBomb(t *testing.T) {
	// 声明 100000x100000 但几乎没有像素数据:真正解码才能证伪
	bomb := []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0, 0, 0, 13, 'I', 'H', 'D', 'R',
		0, 1, 0x86, 0xa0, // width 100000
		0, 1, 0x86, 0xa0, // height 100000
		8, 6, 0, 0, 0,
		0, 0, 0, 0, // bad crc, never reaches pixel data
	}

	if _, err := Validate(bomb, Limits{MaxBytes: 1024 * 1024, MaxDimension: 1024}); err == nil {
		t.Fatal("expected error for oversized declared dimensions, got nil")
	}
}

func TestSniffContentType(t *testing.T) {
	if ct, ok := SniffContentType(encodePNG(t, 2, 2)); !ok || ct != MimeTypePNG {
		t.Fatalf("expected png sniff, got %q ok=%v", ct, ok)
	}
	if ct, ok := SniffContentType(encodeJPEG(t, 2, 2)); !ok || ct != MimeTypeJPEG {
		t.Fatalf("expected jpeg sniff, got %q ok=%v", ct, ok)
	}
	if _, ok := SniffContentType([]byte("nope")); ok {
		t.Fatal("expected unknown content type")
	}
}
