package stylepreview

import "testing"

func TestPreviewKey(t *testing.T) {
	t.Log(fmtPreviewKey("artifact-preview/slides/v1", "cute-v1.webp"))
	t.Log(fmtPreviewKey("artifact-preview/video_overview/v1", "cute.webp"))
}
