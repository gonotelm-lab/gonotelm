package contract

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type StorageResult struct {
	StoreKey    valobj.StoreKey `json:"store_key"`
	ContentType string          `json:"content_type"`
}
