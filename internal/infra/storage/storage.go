package storage

import (
	"context"
	"io"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

var ErrObjectNotFound = errors.New("object not found")

type ObjectInfo struct {
	Key             string
	LastModified    time.Time
	Size            int64 // size in bytes
	ContentType     string
	ContentEncoding string
}

type Provider interface {
	// Provider name
	Name() string
}

type ObjectGetter interface {
	// 检查对象是否存在
	// 返回 ErrObjectNotFound 表示对象不存在
	StatObject(ctx context.Context, req *StatObjectRequest) (*StatObjectResponse, error)

	// 一次性获取对象全部内容
	GetObject(ctx context.Context, req *GetObjectRequest) (*GetObjectResponse, error)
}

type ObjectDeleter interface {
	// 删除对象
	DeleteObject(ctx context.Context, req *DeleteObjectRequest) error
	// 批量删除对象
	BatchDeleteObject(ctx context.Context, req *BatchDeleteObjectRequest) error
}

type ObjectUploader interface {
	// 上传对象
	UploadObject(ctx context.Context, req *UploadObjectRequest) error
}

// 对象存储通用接口 底层可有多种对象存储实现
type Storage interface {
	Provider
	ObjectGetter
	ObjectDeleter
	ObjectUploader

	// 获取Post Policy的预签名上传链接
	PresignedPostPolicy(ctx context.Context, req *PresignedPostPolicyRequest) (*PresignedPostPolicyResponse, error)

	// 获取预签名的下载链接
	PresignedGetObject(ctx context.Context, req *PresignedGetObjectRequest) (*PresignedGetObjectResponse, error)
}
type StatObjectRequest struct {
	Key string
}

type StatObjectResponse struct {
	ObjectInfo
}

type PresignedPostPolicyRequest struct {
	Key           string
	ContentType   string
	ContentLength int64
	Filename      string
	Md5           string
	Metadata      map[string]string
}

type PresignedPostPolicyResponse struct {
	Method  string
	Url     string
	Forms   map[string]string
	Headers map[string]string
}

type PresignedGetObjectRequest struct {
	Key string

	// Inline indicates browser-friendly preview response disposition.
	//
	// Either Inline or Attachment is set. If both are set, Attachment takes precedence.
	Inline bool

	// Attachment indicates download response disposition.
	// Content-Disposition: attachment; filename="filename.ext"
	//
	// Either Inline or Attachment is set. If both are set, Attachment takes precedence.
	Attachment bool

	// set content-disposition filename
	AttachmentFilename string

	// ContentType overrides response content type for presigned preview.
	ContentType string
}

type PresignedGetObjectResponse struct {
	Url string
}

type GetObjectRequest struct {
	Key string
}

type GetObjectResponse struct {
	Body []byte
	Info ObjectInfo
}

type DeleteObjectRequest struct {
	Key string
}

type BatchDeleteObjectRequest struct {
	Keys []string
}

type UploadObjectRequest struct {
	Key string

	// 和BodyReader参数二选一
	Body []byte

	// 和Bod参数二选一
	BodyReader io.Reader

	// 可选: 上传时设置对象内容类型
	ContentType string

	// 可选: 上传时设置用户自定义元信息
	Metadata map[string]string
}

type UploadObjectResponse struct {
	Url string
}
