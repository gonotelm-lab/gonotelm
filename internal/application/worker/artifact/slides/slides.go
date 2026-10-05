package slides

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type SlidesStorageResult struct {
	StoreKey    valobj.StoreKey `json:"store_key"`
	ContentType string          `json:"content_type"`
}
