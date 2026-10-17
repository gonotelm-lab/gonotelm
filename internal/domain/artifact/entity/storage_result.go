package entity

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

// StorageResult 是 storage 类产物结果的信封：worker 产出、notelm 消费。
type StorageResult struct {
	StoreKey    valobj.StoreKey `json:"store_key"`
	ContentType string          `json:"content_type"`
}

func ParseStorageResult(data []byte) (StorageResult, bool) {
	var sr StorageResult
	if err := sonic.Unmarshal(data, &sr); err != nil || !sr.StoreKey.Valid() {
		return StorageResult{}, false
	}

	return sr, true
}
