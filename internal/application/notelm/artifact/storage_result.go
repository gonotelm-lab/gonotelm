package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

// ArtifactView 应用层对外的产物视图，Storage 类结果已带可访问 ContentUrl。
type ArtifactView struct {
	Artifact   *artifactentity.Artifact
	ContentUrl string
	MimeType   string
}

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

func toArtifactView(ctx context.Context, storage adapter.ObjectStore, a *artifactentity.Artifact) *ArtifactView {
	if a == nil {
		return nil
	}
	view := &ArtifactView{Artifact: a}
	if a.ResultKind.Storage() && len(a.Result) > 0 {
		view.ContentUrl, view.MimeType = materializeStorageResult(ctx, storage, a.Result)
	}
	return view
}
