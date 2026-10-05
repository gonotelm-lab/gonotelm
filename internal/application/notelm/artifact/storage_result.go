package artifact

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/contract"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

func extractStoreKey(result []byte) valobj.StoreKey {
	var sr contract.StorageResult
	if err := sonic.Unmarshal(result, &sr); err != nil {
		return valobj.StoreKey{}
	}
	return sr.StoreKey
}

func materializeStorageResult(ctx context.Context, storage adapter.ObjectStore, result []byte) (url string, mime string) {
	if storage == nil || len(result) == 0 {
		return "", ""
	}
	var sr contract.StorageResult
	if err := sonic.Unmarshal(result, &sr); err != nil || !sr.StoreKey.Valid() {
		return "", ""
	}
	url, err := storage.PresignGet(ctx, sr.StoreKey)
	if err != nil {
		return "", sr.ContentType
	}
	return url, sr.ContentType
}
