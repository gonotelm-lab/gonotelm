package artifact

import (
	"context"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/infographic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStorage struct {
	presigned string
	key       string
	err       error
}

func (f *fakeStorage) DeleteObject(ctx context.Context, key string) error { return nil }
func (f *fakeStorage) PresignGet(ctx context.Context, key string) (string, error) {
	f.key = key
	return f.presigned, f.err
}

var _ StorageGateway = &fakeStorage{}

func TestMaterializeStorageResult_FromWorkerOutput(t *testing.T) {
	sr := infographic.StorageResult{
		StoreKey:    "artifact/nb1/t1.png",
		ContentType: "image/png",
		Image:       &infographic.StorageResultImage{Width: 768, Height: 1024},
	}
	bytes, err := sonic.Marshal(sr)
	require.NoError(t, err)

	storage := &fakeStorage{presigned: "https://signed.example.com/t1.png"}

	gotURL, gotMime := materializeStorageResult(context.Background(), storage, bytes)
	assert.Equal(t, "https://signed.example.com/t1.png", gotURL)
	assert.Equal(t, "image/png", gotMime)
	assert.Equal(t, "artifact/nb1/t1.png", storage.key, "should presign the store_key from worker output")

	var broken struct {
		StoreKey    string `json:"StoreKey"`
		ContentType string `json:"ContentType"`
	}
	_ = sonic.Unmarshal(bytes, &broken)
	assert.Empty(t, broken.StoreKey, "PascalCase tags should leave StoreKey empty (old bug)")
	assert.Empty(t, broken.ContentType, "PascalCase tags should leave ContentType empty (old bug)")
}

func TestMaterializeStorageResult_NoStorage(t *testing.T) {
	gotURL, gotMime := materializeStorageResult(context.Background(), nil, []byte(`{"store_key":"x","content_type":"y"}`))
	assert.Empty(t, gotURL)
	assert.Empty(t, gotMime)
}
