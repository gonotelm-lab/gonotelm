package adapter

import (
	"context"
	"io"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

var (
	_ adapter.ObjectStore     = (*ObjectStoreImpl)(nil)
	_ adapter.StoreKeyFactory = (*StoreKeyFactoryImpl)(nil)
)

// ObjectStoreImpl 持有"桶名 → 存储实例"的路由表。
type ObjectStoreImpl struct {
	stores map[string]storage.Storage
}

func NewObjectStore(priv storage.Storage, pub storage.PublicReadStorage) *ObjectStoreImpl {
	stores := map[string]storage.Storage{priv.Bucket(): priv}
	if pub != nil {
		stores[pub.Bucket()] = pub
	}

	return &ObjectStoreImpl{stores: stores}
}

func (s *ObjectStoreImpl) resolve(key valobj.StoreKey) (storage.Storage, error) {
	if !key.Valid() {
		return nil, errors.ErrParams.Msgf("storekey: invalid key %s", key)
	}

	st, ok := s.stores[key.Bucket]
	if !ok {
		return nil, errors.ErrParams.Msgf("storekey: unknown bucket %q", key.Bucket)
	}

	return st, nil
}

func (s *ObjectStoreImpl) Upload(
	ctx context.Context,
	key valobj.StoreKey,
	content []byte,
	contentType string,
) error {
	st, err := s.resolve(key)
	if err != nil {
		return err
	}

	err = st.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         key.Key,
		Body:        content,
		ContentType: contentType,
	})
	if err != nil {
		return errors.WithMessagef(err, "upload object failed, key=%s", key)
	}

	return nil
}

// UploadReader 流式上传，不关闭 src。
func (s *ObjectStoreImpl) UploadReader(
	ctx context.Context,
	key valobj.StoreKey,
	contentType string,
	src io.Reader,
) error {
	st, err := s.resolve(key)
	if err != nil {
		return err
	}

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		defer pw.Close()
		_, copyErr := io.Copy(pw, src)
		select {
		case errCh <- copyErr:
		case <-ctx.Done():
		}
	}()

	uploadErr := st.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         key.Key,
		BodyReader:  pr,
		ContentType: contentType,
	})

	var copyErr error
	select {
	case copyErr = <-errCh:
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			return errors.WithStack(err)
		}
	}

	if copyErr != nil {
		return errors.Wrap(copyErr, "copy source to upload pipe failed")
	}
	if uploadErr != nil {
		return errors.Wrapf(uploadErr, "upload object failed, key=%s", key)
	}

	return nil
}

func (s *ObjectStoreImpl) PresignUpload(
	ctx context.Context,
	key valobj.StoreKey,
	opts *adapter.UploadOptions,
) (*adapter.PresignUploadResult, error) {
	st, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	if opts == nil {
		opts = &adapter.UploadOptions{}
	}

	presignResult, err := st.PresignedPostPolicy(ctx, &storage.PresignedPostPolicyRequest{
		Key:           key.Key,
		ContentType:   opts.ContentType,
		ContentLength: opts.ContentLength,
		Filename:      opts.Filename,
		Md5:           opts.Md5,
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "presign upload object failed, key=%s", key)
	}

	return &adapter.PresignUploadResult{
		Method:  presignResult.Method,
		Url:     presignResult.Url,
		Forms:   presignResult.Forms,
		Headers: presignResult.Headers,
	}, nil
}

func (s *ObjectStoreImpl) PresignGet(ctx context.Context, key valobj.StoreKey) (string, error) {
	st, err := s.resolve(key)
	if err != nil {
		return "", err
	}

	presignResult, err := st.PresignedGetObject(ctx, &storage.PresignedGetObjectRequest{
		Key: key.Key,
	})
	if err != nil {
		return "", errors.WithMessagef(err, "presign get object failed, key=%s", key)
	}

	return presignResult.Url, nil
}

func (s *ObjectStoreImpl) CheckExist(ctx context.Context, key valobj.StoreKey) (bool, error) {
	st, err := s.resolve(key)
	if err != nil {
		return false, err
	}

	_, err = st.StatObject(ctx, &storage.StatObjectRequest{Key: key.Key})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return false, nil
		}

		return false, errors.WithMessagef(err, "check object exist failed, key=%s", key)
	}

	return true, nil
}

func (s *ObjectStoreImpl) GetObject(
	ctx context.Context,
	key valobj.StoreKey,
) ([]byte, *adapter.ObjectInfo, error) {
	st, err := s.resolve(key)
	if err != nil {
		return nil, nil, err
	}

	object, err := st.GetObject(ctx, &storage.GetObjectRequest{Key: key.Key})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, nil, adapter.ErrObjectNotFound
		}

		return nil, nil, errors.WithMessagef(err, "get object failed, key=%s", key)
	}

	return object.Body, objectInfoOf(&object.Info), nil
}

func (s *ObjectStoreImpl) GetPartialObject(
	ctx context.Context,
	key valobj.StoreKey,
	offset int64,
	length int64,
) ([]byte, *adapter.ObjectInfo, error) {
	st, err := s.resolve(key)
	if err != nil {
		return nil, nil, err
	}

	object, err := st.GetPartialObject(ctx, &storage.GetPartialObjectRequest{
		Key:    key.Key,
		Offset: offset,
		Length: length,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, nil, adapter.ErrObjectNotFound
		}

		return nil, nil, errors.WithMessagef(err, "get partial object failed, key=%s", key)
	}

	return object.Body, objectInfoOf(&object.Info), nil
}

func (s *ObjectStoreImpl) DeleteObject(ctx context.Context, key valobj.StoreKey) error {
	st, err := s.resolve(key)
	if err != nil {
		return err
	}

	err = st.DeleteObject(ctx, &storage.DeleteObjectRequest{Key: key.Key})
	if err != nil {
		return errors.WithMessagef(err, "delete object failed, key=%s", key)
	}

	return nil
}

func (s *ObjectStoreImpl) BatchDeleteObject(ctx context.Context, keys []valobj.StoreKey) error {
	if len(keys) == 0 {
		return nil
	}

	// 一次批量删除可能横跨多个桶
	grouped := make(map[string][]string)
	for _, key := range keys {
		if !key.Valid() {
			continue
		}
		grouped[key.Bucket] = append(grouped[key.Bucket], key.Key)
	}

	for bucket, objectKeys := range grouped {
		st, ok := s.stores[bucket]
		if !ok {
			return errors.ErrParams.Msgf("storekey: unknown bucket %q", bucket)
		}

		err := st.BatchDeleteObject(ctx, &storage.BatchDeleteObjectRequest{Keys: objectKeys})
		if err != nil {
			return errors.WithMessagef(err, "batch delete objects failed, bucket=%s", bucket)
		}
	}

	return nil
}

func (s *ObjectStoreImpl) PublicURL(ctx context.Context, key valobj.StoreKey) (string, error) {
	if !key.IsPublic {
		return "", errors.ErrParams.Msgf("storekey: object is not public-read, key=%s", key)
	}

	st, err := s.resolve(key)
	if err != nil {
		return "", err
	}

	publicStore, ok := st.(storage.PublicReadStorage)
	if !ok {
		return "", errors.ErrParams.Msgf("bucket %q does not support public url", key.Bucket)
	}

	return publicStore.GetObjectUrl(ctx, key.Key), nil
}

func objectInfoOf(info *storage.ObjectInfo) *adapter.ObjectInfo {
	if info == nil {
		return nil
	}

	return &adapter.ObjectInfo{
		Key:         info.Key,
		Size:        info.Size,
		ContentType: info.ContentType,
	}
}

// StoreKeyFactoryImpl 从部署配置取桶名；public 桶未配置时 isPublic=true 直接报错。
type StoreKeyFactoryImpl struct {
	privateBucket string
	publicBucket  string
}

func NewStoreKeyFactory(priv storage.Storage, pub storage.PublicReadStorage) *StoreKeyFactoryImpl {
	f := &StoreKeyFactoryImpl{privateBucket: priv.Bucket()}
	if pub != nil {
		f.publicBucket = pub.Bucket()
	}

	return f
}

func (f *StoreKeyFactoryImpl) New(path string, isPublic bool) (valobj.StoreKey, error) {
	bucket := f.privateBucket
	if isPublic {
		if f.publicBucket == "" {
			return valobj.StoreKey{}, errors.ErrParams.Msg(
				"storekey: public bucket is not configured")
		}
		bucket = f.publicBucket
	}

	return valobj.NewStoreKey(bucket, path, isPublic)
}
