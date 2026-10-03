package infographic

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type StorageResult struct {
	StoreKey    valobj.StoreKey     `json:"store_key"`
	ContentType string              `json:"content_type"`
	Image       *StorageResultImage `json:"image,omitempty"`
}

type StorageResultImage struct {
	Width  int `json:"w"`
	Height int `json:"h"`
}
