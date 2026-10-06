package stylepreview

import "testing"

func TestSlidesPreviewKey(t *testing.T) {
	t.Log(fmtSlidesPreviewKey("v1", "png"))
	t.Log(fmtSlidesPreviewKey("v2", "webp"))
}
