package adapter

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStorage struct {
	bucket string

	uploadedKey         string
	uploadedContentType string
	uploadedBody        []byte
	deletedKeys         []string
	batchDeletedKeys    []string
	presignedKey        string
}

func (f *fakeStorage) Name() string   { return "fake" }
func (f *fakeStorage) Bucket() string { return f.bucket }

func (f *fakeStorage) UploadObject(_ context.Context, req *storage.UploadObjectRequest) error {
	f.uploadedKey = req.Key
	f.uploadedContentType = req.ContentType
	body, err := io.ReadAll(req.BodyReader)
	if err != nil {
		return err
	}
	f.uploadedBody = body
	return nil
}

func (f *fakeStorage) DeleteObject(_ context.Context, req *storage.DeleteObjectRequest) error {
	f.deletedKeys = append(f.deletedKeys, req.Key)
	return nil
}

func (f *fakeStorage) BatchDeleteObject(_ context.Context, req *storage.BatchDeleteObjectRequest) error {
	f.batchDeletedKeys = append(f.batchDeletedKeys, req.Keys...)
	return nil
}

func (f *fakeStorage) PresignedGetObject(_ context.Context, req *storage.PresignedGetObjectRequest) (*storage.PresignedGetObjectResponse, error) {
	f.presignedKey = req.Key
	return &storage.PresignedGetObjectResponse{Url: "https://signed/" + req.Key}, nil
}

func (f *fakeStorage) StatObject(context.Context, *storage.StatObjectRequest) (*storage.StatObjectResponse, error) {
	return nil, storage.ErrObjectNotFound
}
func (f *fakeStorage) GetObject(context.Context, *storage.GetObjectRequest) (*storage.GetObjectResponse, error) {
	return nil, storage.ErrObjectNotFound
}
func (f *fakeStorage) GetPartialObject(context.Context, *storage.GetPartialObjectRequest) (*storage.GetPartialObjectResponse, error) {
	return nil, storage.ErrObjectNotFound
}
func (f *fakeStorage) PresignedPostPolicy(context.Context, *storage.PresignedPostPolicyRequest) (*storage.PresignedPostPolicyResponse, error) {
	return &storage.PresignedPostPolicyResponse{}, nil
}

var _ storage.Storage = (*fakeStorage)(nil)

// fakePublicStorage 只有公有读桶的实例才具备直链能力。
type fakePublicStorage struct {
	*fakeStorage
}

func (f *fakePublicStorage) GetObjectUrl(_ context.Context, key string) string {
	return "https://public/" + f.bucket + "/" + key
}

var (
	_ storage.Storage           = (*fakePublicStorage)(nil)
	_ storage.PublicReadStorage = (*fakePublicStorage)(nil)
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func newTestKey(t *testing.T, bucket, path string) valobj.StoreKey {
	t.Helper()
	key, err := valobj.NewStoreKey(bucket, path, false)
	require.NoError(t, err)
	return key
}

func TestUploadReader_StreamsBody(t *testing.T) {
	priv := &fakeStorage{bucket: "gonotelm"}
	store := NewObjectStore(priv, nil)

	key := newTestKey(t, "gonotelm", "artifact/a/b.pptx")
	err := store.UploadReader(context.Background(), key, "application/octet-stream", strings.NewReader("hello-pptx"))
	require.NoError(t, err)

	assert.Equal(t, "artifact/a/b.pptx", priv.uploadedKey)
	assert.Equal(t, "application/octet-stream", priv.uploadedContentType)
	assert.Equal(t, []byte("hello-pptx"), priv.uploadedBody)
}

func TestUploadReader_PropagatesCopyError(t *testing.T) {
	priv := &fakeStorage{bucket: "gonotelm"}
	store := NewObjectStore(priv, nil)

	err := store.UploadReader(context.Background(), newTestKey(t, "gonotelm", "k"), "ct", errReader{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "copy source to upload pipe failed")
}

func TestUploadReader_DoesNotCloseSource(t *testing.T) {
	priv := &fakeStorage{bucket: "gonotelm"}
	store := NewObjectStore(priv, nil)

	src := io.NopCloser(bytes.NewReader([]byte("x")))
	require.NoError(t, store.UploadReader(context.Background(), newTestKey(t, "gonotelm", "k"), "ct", src))
	assert.Equal(t, []byte("x"), priv.uploadedBody)
}

func TestObjectStore_RoutesByBucket(t *testing.T) {
	priv := &fakeStorage{bucket: "gonotelm-private"}
	pub := &fakePublicStorage{fakeStorage: &fakeStorage{bucket: "gonotelm-public"}}
	store := NewObjectStore(priv, pub)

	_, err := store.PresignGet(context.Background(), newTestKey(t, "gonotelm-private", "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "a.txt", priv.presignedKey)
	assert.Empty(t, pub.presignedKey)

	priv.presignedKey = ""
	_, err = store.PresignGet(context.Background(), newTestKey(t, "gonotelm-public", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "b.txt", pub.presignedKey)
	assert.Empty(t, priv.presignedKey, "private instance must not serve the public bucket")
}

func TestObjectStore_UnknownBucket(t *testing.T) {
	store := NewObjectStore(&fakeStorage{bucket: "gonotelm-private"}, nil)

	_, err := store.PresignGet(context.Background(), newTestKey(t, "other-bucket", "a.txt"))
	assert.Error(t, err)
}

func TestBatchDeleteObject_GroupsByBucket(t *testing.T) {
	priv := &fakeStorage{bucket: "gonotelm-private"}
	pub := &fakePublicStorage{fakeStorage: &fakeStorage{bucket: "gonotelm-public"}}
	store := NewObjectStore(priv, pub)

	err := store.BatchDeleteObject(context.Background(), []valobj.StoreKey{
		newTestKey(t, "gonotelm-private", "a.txt"),
		newTestKey(t, "gonotelm-public", "b.txt"),
		newTestKey(t, "gonotelm-private", "c.txt"),
	})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"a.txt", "c.txt"}, priv.batchDeletedKeys)
	assert.ElementsMatch(t, []string{"b.txt"}, pub.batchDeletedKeys)
}

func TestStoreKeyFactory_PrivateAndPublic(t *testing.T) {
	f := NewStoreKeyFactory(&fakeStorage{bucket: "gonotelm-private"}, &fakePublicStorage{fakeStorage: &fakeStorage{bucket: "gonotelm-public"}})

	key, err := f.New("file/nb/a.pdf", false)
	require.NoError(t, err)
	assert.Equal(t, "gonotelm-private", key.Bucket)

	key, err = f.New("file/nb/a.pdf", true)
	require.NoError(t, err)
	assert.Equal(t, "gonotelm-public", key.Bucket)
	assert.True(t, key.IsPublic)
}

func TestStoreKeyFactory_PublicBucketNotConfigured(t *testing.T) {
	f := NewStoreKeyFactory(&fakeStorage{bucket: "gonotelm-private"}, nil)

	_, err := f.New("file/nb/a.pdf", true)
	assert.Error(t, err, "isPublic=true 不能在未配置公有桶时静默落到私有桶")
}

func TestObjectStore_PublicURL(t *testing.T) {
	priv := &fakeStorage{bucket: "gonotelm-private"}
	pub := &fakePublicStorage{fakeStorage: &fakeStorage{bucket: "gonotelm-public"}}
	store := NewObjectStore(priv, pub)

	publicKey, err := valobj.NewStoreKey("gonotelm-public", "users/avatar/abc", true)
	require.NoError(t, err)

	url, err := store.PublicURL(context.Background(), publicKey)
	require.NoError(t, err)
	assert.Equal(t, "https://public/gonotelm-public/users/avatar/abc", url)
}

func TestObjectStore_PublicURL_RejectsPrivateBucket(t *testing.T) {
	priv := &fakeStorage{bucket: "gonotelm-private"}
	store := NewObjectStore(priv, nil)

	_, err := store.PublicURL(context.Background(), newTestKey(t, "gonotelm-private", "users/avatar/abc"))
	assert.Error(t, err, "私有对象没有稳定直链")
}

func TestObjectStore_PublicURL_RequiresIsPublic(t *testing.T) {
	// 生产里 minio 实例无论绑定哪个桶都实现 GetObjectUrl，所以必须靠 IsPublic 拦住
	priv := &fakeStorage{bucket: "gonotelm-private"}
	store := NewObjectStore(priv, nil)

	key, err := valobj.NewStoreKey("gonotelm-private", "users/avatar/abc", false)
	require.NoError(t, err)

	_, err = store.PublicURL(context.Background(), key)
	assert.Error(t, err)
}

func TestObjectStore_PublicURL_InstanceWithoutCapability(t *testing.T) {
	// IsPublic=true 但解析到的实例没有直链能力（私有 fake 不实现 GetObjectUrl）
	store := NewObjectStore(&fakeStorage{bucket: "gonotelm-public"}, nil)

	key, err := valobj.NewStoreKey("gonotelm-public", "users/avatar/abc", true)
	require.NoError(t, err)

	_, err = store.PublicURL(context.Background(), key)
	assert.Error(t, err)
}
