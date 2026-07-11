package artifact

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/infographic"
)

func extractStoreKey(result []byte) string {
	var sr infographic.StorageResult
	if err := sonic.Unmarshal(result, &sr); err != nil {
		return ""
	}
	return sr.StoreKey
}

func materializeStorageResult(ctx context.Context, storage StorageGateway, result []byte) (url string, mime string) {
	if storage == nil || len(result) == 0 {
		return "", ""
	}
	var sr infographic.StorageResult
	if err := sonic.Unmarshal(result, &sr); err != nil || sr.StoreKey == "" {
		return "", ""
	}
	url, err := storage.PresignGet(ctx, sr.StoreKey)
	if err != nil {
		return "", sr.ContentType
	}
	return url, sr.ContentType
}
