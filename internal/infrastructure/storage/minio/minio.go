package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/attributes"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/s3conv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Storage struct {
	client        *minio.Client
	bucket        string
	region        string
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

	// 自带重试+链路追踪的httpclient
	transport := httpclient.NewBuilder(nil).Build().Transport
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:      credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:     cfg.Secure,
		Region:     cfg.Region,
		Transport:  transport,
		MaxRetries: 1,
	})
	if err != nil {
		return nil, errors.Wrap(err, "create minio client failed")
	}

	return &Storage{
		client:        client,
		bucket:        cfg.Bucket,
		region:        cfg.Region,
		presignExpiry: cfg.PresignExpiry,
	}, nil
}

func (s *Storage) Name() string {
	return "minio"
}

// startSpan 为一次存储操作创建 S3 风格 client span，
// 属性由 s3conv 生成，对齐 AWS SDK v2 的 otelaws instrumentation。
func (s *Storage) startSpan(ctx context.Context, op, key string,
	extra ...attribute.KeyValue,
) (context.Context, oteltrace.Span) {
	attrs := s3conv.Attributes(op, s.bucket, s.region, key)
	attrs = append(attrs, extra...)

	return pkgtrace.GetOtelTracer().Start(ctx, s3conv.SpanName(op),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attrs...),
	)
}

func (s *Storage) StatObject(
	ctx context.Context,
	req *storage.StatObjectRequest,
) (*storage.StatObjectResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	ctx, span := s.startSpan(ctx, "StatObject", req.Key)
	defer span.End()

	objInfo, err := s.client.StatObject(ctx, s.bucket, req.Key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFoundErr(err) {
			err = storage.ErrObjectNotFound
		} else {
			err = errors.Wrapf(err, "minio stat object failed, key=%s", req.Key)
		}
		finishSpan(span, err)
		return nil, err
	}

	return &storage.StatObjectResponse{
		ObjectInfo: storage.ObjectInfo{
			Key:             objInfo.Key,
			LastModified:    objInfo.LastModified,
			Size:            objInfo.Size,
			ContentType:     objInfo.ContentType,
			ContentEncoding: objInfo.ContentEncoding,
		},
	}, nil
}

func (s *Storage) GetObject(
	ctx context.Context,
	req *storage.GetObjectRequest,
) (*storage.GetObjectResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	ctx, span := s.startSpan(ctx, "GetObject", req.Key)
	defer span.End()

	object, err := s.client.GetObject(ctx, s.bucket, req.Key, minio.GetObjectOptions{})
	if err != nil {
		if isNotFoundErr(err) {
			err = storage.ErrObjectNotFound
		} else {
			err = errors.Wrap(err, "get object failed")
		}
		finishSpan(span, err)
		return nil, err
	}

	objInfo, err := object.Stat()
	if err != nil {
		if isNotFoundErr(err) {
			err = storage.ErrObjectNotFound
		} else {
			err = errors.Wrap(err, "get stat object failed")
		}
		finishSpan(span, err)
		return nil, err
	}

	body, err := io.ReadAll(object)
	if err != nil {
		err = errors.Wrap(err, "read object body failed")
		finishSpan(span, err)
		return nil, err
	}
	defer object.Close()

	return &storage.GetObjectResponse{
		Body: body,
		Info: makeObjectInfo(objInfo),
	}, nil
}

func (s *Storage) GetPartialObject(
	ctx context.Context,
	req *storage.GetPartialObjectRequest,
) (*storage.GetPartialObjectResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	ctx, span := s.startSpan(ctx, "GetPartialObject", req.Key)
	defer span.End()

	objInfo, err := s.client.StatObject(ctx, s.bucket, req.Key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFoundErr(err) {
			err = storage.ErrObjectNotFound
		} else {
			err = errors.Wrapf(err, "get stat object failed")
		}
		finishSpan(span, err)
		return nil, err
	}
	if req.Offset >= objInfo.Size {
		err = errors.Wrapf(io.EOF, "read partial object failed, key=%s, offset=%d, length=%d",
			req.Key, req.Offset, req.Length)
		finishSpan(span, err)
		return nil, err
	}

	end := req.Offset + req.Length - 1
	if end >= objInfo.Size {
		end = objInfo.Size - 1
	}

	opts := minio.GetObjectOptions{}
	if err := opts.SetRange(req.Offset, end); err != nil {
		err = errors.Wrapf(err, "set partial object range failed, key=%s, offset=%d, length=%d",
			req.Key, req.Offset, req.Length)
		finishSpan(span, err)
		return nil, err
	}

	object, err := s.client.GetObject(ctx, s.bucket, req.Key, opts)
	if err != nil {
		if isNotFoundErr(err) {
			err = storage.ErrObjectNotFound
		} else {
			err = errors.Wrapf(err, "get partial object failed, key=%s, offset=%d, length=%d",
				req.Key, req.Offset, req.Length)
		}
		finishSpan(span, err)
		return nil, err
	}
	defer object.Close()

	body, err := io.ReadAll(object)
	if err != nil {
		err = errors.Wrapf(err, "read partial object failed, key=%s, offset=%d, length=%d",
			req.Key, req.Offset, req.Length)
		finishSpan(span, err)
		return nil, err
	}

	return &storage.GetPartialObjectResponse{
		Body:      body,
		BytesRead: len(body),
		Info:      makeObjectInfo(objInfo),
	}, nil
}

func (s *Storage) DeleteObject(
	ctx context.Context,
	req *storage.DeleteObjectRequest,
) error {
	if err := req.Validate(); err != nil {
		return err
	}

	ctx, span := s.startSpan(ctx, "DeleteObject", req.Key)
	defer span.End()

	err := s.client.RemoveObject(ctx, s.bucket, req.Key, minio.RemoveObjectOptions{})
	if err != nil {
		err = errors.Wrapf(err, "minio delete object failed, key=%s", req.Key)
		finishSpan(span, err)
		return err
	}

	return nil
}

func (s *Storage) BatchDeleteObject(
	ctx context.Context,
	req *storage.BatchDeleteObjectRequest,
) error {
	if err := req.Validate(); err != nil {
		return err
	}
	if len(req.Keys) == 0 {
		return nil
	}

	ctx, span := s.startSpan(ctx, "DeleteObjects", "", attributes.S3ObjectCount.Int(len(req.Keys)))
	defer span.End()

	objectCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectCh)
		for _, key := range req.Keys {
			if key == "" {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case objectCh <- minio.ObjectInfo{Key: key}:
			}
		}
	}()

	errCh := s.client.RemoveObjects(ctx, s.bucket, objectCh, minio.RemoveObjectsOptions{})
	for rmErr := range errCh {
		if rmErr.Err != nil {
			err := errors.Wrapf(rmErr.Err, "minio batch delete object failed, key=%s", rmErr.ObjectName)
			finishSpan(span, err)
			return err
		}
	}

	return nil
}

func (s *Storage) UploadObject(
	ctx context.Context,
	req *storage.UploadObjectRequest,
) error {
	if err := req.Validate(); err != nil {
		return err
	}

	ctx, span := s.startSpan(ctx, "PutObject", req.Key)
	defer span.End()

	var (
		reader io.Reader
		size   int64
	)
	if req.Body != nil {
		reader = bytes.NewReader(req.Body)
		size = int64(len(req.Body))
	} else {
		reader = req.BodyReader
		size = -1
	}

	_, err := s.client.PutObject(
		ctx,
		s.bucket,
		req.Key,
		reader,
		size,
		minio.PutObjectOptions{
			ContentType:  req.ContentType,
			UserMetadata: req.Metadata,
		})
	if err != nil {
		err = errors.Wrapf(err, "minio upload object failed, key=%s", req.Key)
		finishSpan(span, err)
		return err
	}

	return nil
}

func (s *Storage) PresignedPostPolicy(
	ctx context.Context,
	req *storage.PresignedPostPolicyRequest,
) (*storage.PresignedPostPolicyResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	ctx, span := s.startSpan(ctx, "PresignedPostPolicy", req.Key)
	defer span.End()

	policy := minio.NewPostPolicy()
	if err := policy.SetBucket(s.bucket); err != nil {
		err = errors.Wrapf(err, "set post policy bucket failed, bucket=%s", s.bucket)
		finishSpan(span, err)
		return nil, err
	}
	if err := policy.SetKey(req.Key); err != nil {
		err = errors.Wrapf(err, "set post policy key failed, key=%s", req.Key)
		finishSpan(span, err)
		return nil, err
	}
	if err := policy.SetExpires(time.Now().UTC().Add(s.presignExpiry)); err != nil {
		err = errors.Wrapf(err, "set post policy expiry failed, expiry=%s", s.presignExpiry)
		finishSpan(span, err)
		return nil, err
	}
	if req.ContentType != "" {
		if err := policy.SetContentType(req.ContentType); err != nil {
			err = errors.Wrapf(err, "set post policy content type failed, content_type=%s", req.ContentType)
			finishSpan(span, err)
			return nil, err
		}
	}
	if req.ContentLength > 0 {
		if err := policy.SetContentLengthRange(req.ContentLength, req.ContentLength); err != nil {
			err = errors.Wrapf(err, "set post policy content length failed, content_length=%d", req.ContentLength)
			finishSpan(span, err)
			return nil, err
		}
	}
	if req.Filename != "" {
		if err := policy.SetUserMetadata("filename", req.Filename); err != nil {
			err = errors.Wrapf(err, "set post policy filename metadata failed, filename=%s", req.Filename)
			finishSpan(span, err)
			return nil, err
		}
	}
	if req.Md5 != "" {
		if err := policy.SetUserMetadata("md5", req.Md5); err != nil {
			err = errors.Wrapf(err, "set post policy md5 metadata failed, md5=%s", req.Md5)
			finishSpan(span, err)
			return nil, err
		}
	}

	for k, v := range req.Metadata {
		if k == "" {
			continue
		}
		if err := policy.SetUserMetadata(k, v); err != nil {
			err = errors.Wrapf(err, "set post policy metadata failed, key=%s, value=%s", k, v)
			finishSpan(span, err)
			return nil, err
		}
	}

	presignedURL, formData, err := s.client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		err = errors.Wrap(err, "generate minio presigned post policy failed")
		finishSpan(span, err)
		return nil, err
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
	if err := req.Validate(); err != nil {
		return nil, err
	}

	ctx, span := s.startSpan(ctx, "PresignedGetObject", req.Key)
	defer span.End()

	params := url.Values{}
	if req.Inline {
		params.Set("response-content-disposition", "inline")
	}
	if req.Attachment {
		disposition := "attachment"
		if req.AttachmentFilename != "" {
			disposition += fmt.Sprintf(`; filename="%s"`, req.AttachmentFilename)
		}
		params.Set("response-content-disposition", disposition)
	}

	if req.ContentType != "" {
		params.Set("response-content-type", req.ContentType)
	}

	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, req.Key, s.presignExpiry, params)
	if err != nil {
		err = errors.Wrapf(err, "generate minio presigned get object failed, key=%s", req.Key)
		finishSpan(span, err)
		return nil, err
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

// finishSpan 在操作失败时记录错误并标记 span 状态。
func finishSpan(span oteltrace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

func makeObjectInfo(objInfo minio.ObjectInfo) storage.ObjectInfo {
	return storage.ObjectInfo{
		Key:             objInfo.Key,
		LastModified:    objInfo.LastModified,
		Size:            objInfo.Size,
		ContentType:     objInfo.ContentType,
		ContentEncoding: objInfo.ContentEncoding,
	}
}
