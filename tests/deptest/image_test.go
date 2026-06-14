package deptest

import (
	"bytes"
	"os"
	"testing"

	"image"
	_ "image/png"
	_ "image/jpeg"
)

func TestImageDecode(t *testing.T) {
	img, err := os.ReadFile("./testdata/image.png")
	if err != nil {
		t.Fatalf("read image failed: %v", err)
	}

	conf, a, err := image.DecodeConfig(bytes.NewBuffer(img))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("color model: %v", conf.ColorModel)
	t.Logf("width: %d", conf.Width)
	t.Logf("height: %d", conf.Height)
	t.Logf("format: %s", a)
}
