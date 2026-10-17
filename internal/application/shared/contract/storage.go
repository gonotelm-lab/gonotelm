package contract

import (
	"encoding/json"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type StorageResult struct {
	StoreKey    valobj.StoreKey `json:"store_key"`
	ContentType string          `json:"content_type"`
}

func ParseStorageResult(data []byte) (StorageResult, bool) {
	var sr StorageResult
	if err := json.Unmarshal(data, &sr); err != nil || !sr.StoreKey.Valid() {
		return StorageResult{}, false
	}

	return sr, true
}
