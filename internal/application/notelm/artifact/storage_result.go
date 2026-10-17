package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func extractStoreKey(result []byte) valobj.StoreKey {
	sr, ok := artifactentity.ParseStorageResult(result)
	if !ok {
		return valobj.StoreKey{}
	}

	return sr.StoreKey
}

func materializeStorageResult(ctx context.Context, storage adapter.ObjectStore, result []byte) (url string, mime string) {
	if storage == nil || len(result) == 0 {
		return "", ""
	}

	sr, ok := artifactentity.ParseStorageResult(result)
	if !ok {
		return "", ""
	}

	url, err := storage.PresignGet(ctx, sr.StoreKey)
	if err != nil {
		return "", sr.ContentType
	}
	return url, sr.ContentType
}
