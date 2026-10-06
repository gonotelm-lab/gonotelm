package stylepreview

import (
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

// The preview object key mirrors the assets directory layout:
// <prefix>/<artifact kind>/<generation>/<uuid>.<ext>.
const (
	previewKeyPrefix = "artifact-preview"
	previewKeySlides = "slides"

	// slidesPreviewVersionV1 is the generation marker of the slides preview
	// assets. It is unrelated to the row identifier (which carries no version)
	// and only namespaces the objects: bump it when the asset format changes.
	slidesPreviewVersionV1 = "v1"
)

func fmtSlidesPreviewKey(version string, fileExt string) string {
	// for example: artifact-preview/slides/v1/uuidv4.webp
	k := fmt.Sprintf("%s/%s/%s/%s", previewKeyPrefix, previewKeySlides, version, uuid.NewV4().String())
	if len(fileExt) > 0 {
		if after, ok := strings.CutPrefix(fileExt, "."); ok {
			fileExt = after
		}
	}

	return k + "." + fileExt
}
