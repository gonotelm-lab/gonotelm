package model

import "testing"

func TestSupportedFileMimeType(t *testing.T) {
	supported := []string{
		MimeTypePDF,
		MimeTypeText,
		MimeTypeMarkdown,
		MimeTypeEPUB,
		MimeTypeWord,
	}

	for _, mimeType := range supported {
		if !SupportedFileMimeType(mimeType) {
			t.Fatalf("mime type should be supported: %s", mimeType)
		}
	}

	unsupported := []string{
		"",
		"application/msword",
		"image/png",
		"text/html",
	}

	for _, mimeType := range unsupported {
		if SupportedFileMimeType(mimeType) {
			t.Fatalf("mime type should be unsupported: %s", mimeType)
		}
	}
}
