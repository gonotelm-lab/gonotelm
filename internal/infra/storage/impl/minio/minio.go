package minio

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client        *minio.Client
	bucket        string
	presignExpiry time.Duration
}

var _ storage.Storage = (*Storage)(nil)

func New(cfg *storage.Config) (*Storage, error) {
	if cfg == nil {
		return nil, errors.New("storage config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate storage config failed")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, errors.Wrap(err, "create minio client failed")
	}

	return &Storage{
		client:        client,
		bucket:        cfg.Bucket,
		presignExpiry: cfg.PresignExpiry,
	}, nil
}

func (s *Storage) Name() string {
	return "minio"
}

func (s *Storage) StatObject(
	ctx context.Context,
	req *storage.StatObjectRequest,
) (*storage.StatObjectResponse, error) {
	if req == nil {
		return nil, errors.ErrParams.Msg("stat object request is nil")
	}
	if req.Key == "" {
		return nil, errors.ErrParams.Msg("stat object request key is empty")
	}

	_, err := s.client.StatObject(ctx, s.bucket, req.Key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFoundErr(err) {
			return nil, storage.ErrObjectNotFound
		}
		return nil, errors.Wrapf(err, "minio stat object failed, key=%s", req.Key)
	}

	return &storage.StatObjectResponse{}, nil
}

func (s *Storage) GetObject(
	ctx context.Context,
	req *storage.GetObjectRequest,
) (*storage.GetObjectResponse, error) {
	if req == nil {
		return nil, errors.ErrParams.Msg("get object request is nil")
	}
	if req.Key == "" {
		return nil, errors.ErrParams.Msg("get object request key is empty")
	}

	object, err := s.client.GetObject(ctx, s.bucket, req.Key, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get object failed")
	}

	body, err := io.ReadAll(object)
	if err != nil {
		return nil, errors.Wrap(err, "read object body failed")
	}
	defer object.Close()

	return &storage.GetObjectResponse{
		Body: body,
	}, nil
}

func (s *Storage) DeleteObject(
	ctx context.Context,
	req *storage.DeleteObjectRequest,
) error {
	if req == nil {
		return errors.ErrParams.Msg("delete object request is nil")
	}
	if req.Key == "" {
		return errors.ErrParams.Msg("delete object request key is empty")
	}

	err := s.client.RemoveObject(ctx, s.bucket, req.Key, minio.RemoveObjectOptions{})
	if err != nil {
		return errors.Wrapf(err, "minio delete object failed, key=%s", req.Key)
	}

	return nil
}

func (s *Storage) UploadObject(
	ctx context.Context,
	req *storage.UploadObjectRequest,
) error {
	if req == nil {
		return errors.ErrParams.Msg("upload object request is nil")
	}
	if req.Key == "" {
		return errors.ErrParams.Msg("upload object request key is empty")
	}

	reader := bytes.NewReader(req.Body)
	_, err := s.client.PutObject(
		ctx,
		s.bucket,
		req.Key,
		reader,
		int64(len(req.Body)),
		minio.PutObjectOptions{
			ContentType:  req.ContentType,
			UserMetadata: req.Metadata,
		})
	if err != nil {
		return errors.Wrapf(err, "minio upload object failed, key=%s", req.Key)
	}

	return nil
}

func (s *Storage) PresignedPostPolicy(
	ctx context.Context,
	req *storage.PresignedPostPolicyRequest,
) (*storage.PresignedPostPolicyResponse, error) {
	if req == nil {
		return nil, errors.ErrParams.Msg("presigned post policy request is nil")
	}
	if req.Key == "" {
		return nil, errors.ErrParams.Msg("presigned post policy request key is empty")
	}

	policy := minio.NewPostPolicy()
	if err := policy.SetBucket(s.bucket); err != nil {
		return nil, errors.Wrapf(err, "set post policy bucket failed, bucket=%s", s.bucket)
	}
	if err := policy.SetKey(req.Key); err != nil {
		return nil, errors.Wrapf(err, "set post policy key failed, key=%s", req.Key)
	}
	if err := policy.SetExpires(time.Now().UTC().Add(s.presignExpiry)); err != nil {
		return nil, errors.Wrapf(err, "set post policy expiry failed, expiry=%s", s.presignExpiry)
	}
	if req.ContentType != "" {
		if err := policy.SetContentType(req.ContentType); err != nil {
			return nil, errors.Wrapf(err, "set post policy content type failed, content_type=%s", req.ContentType)
		}
	}
	if req.ContentLength > 0 {
		if err := policy.SetContentLengthRange(req.ContentLength, req.ContentLength); err != nil {
			return nil, errors.Wrapf(err, "set post policy content length failed, content_length=%d", req.ContentLength)
		}
	}
	if req.Filename != "" {
		if err := policy.SetUserMetadata("filename", req.Filename); err != nil {
			return nil, errors.Wrapf(err, "set post policy filename metadata failed, filename=%s", req.Filename)
		}
	}
	if req.Md5 != "" {
		if err := policy.SetUserMetadata("md5", req.Md5); err != nil {
			return nil, errors.Wrapf(err, "set post policy md5 metadata failed, md5=%s", req.Md5)
		}
	}

	for k, v := range req.Metadata {
		if k == "" {
			continue
		}
		if err := policy.SetUserMetadata(k, v); err != nil {
			return nil, errors.Wrapf(err, "set post policy metadata failed, key=%s, value=%s", k, v)
		}
	}

	presignedURL, formData, err := s.client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return nil, errors.Wrap(err, "generate minio presigned post policy failed")
	}

	return &storage.PresignedPostPolicyResponse{
		Method: http.MethodPost,
		Url:    presignedURL.String(),
		Forms:  formData,
	}, nil
}

func (s *Storage) PresignedGetObject(
	ctx context.Context,
	req *storage.PresignedGetObjectRequest,
) (*storage.PresignedGetObjectResponse, error) {
	if req == nil {
		return nil, errors.New("presigned get object request is nil")
	}
	if req.Key == "" {
		return nil, errors.New("presigned get object request key is empty")
	}

	params := url.Values{}
	if req.Inline {
		params.Set("response-content-disposition", "inline")
	}
	if req.ContentType != "" {
		params.Set("response-content-type", req.ContentType)
	}

	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, req.Key, s.presignExpiry, params)
	if err != nil {
		return nil, errors.Wrapf(err, "generate minio presigned get object failed, key=%s", req.Key)
	}

	return &storage.PresignedGetObjectResponse{
		Url: presignedURL.String(),
	}, nil
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}

	code := strings.TrimSpace(minio.ToErrorResponse(err).Code)
	switch code {
	case "NoSuchKey", "NoSuchBucket", "NotFound":
		return true
	default:
		return false
	}
}
