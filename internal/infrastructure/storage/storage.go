package storage

import (
	"context"
	stderrors "errors"
	"io"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

var ErrObjectNotFound = errors.New("object not found")

type ObjectInfo struct {
	Key             string
	LastModified    time.Time
	Size            int64
	ContentType     string
	ContentEncoding string
}

type Provider interface {
	Name() string
}

type ObjectGetter interface {
	StatObject(ctx context.Context, req *StatObjectRequest) (*StatObjectResponse, error)

	GetObject(ctx context.Context, req *GetObjectRequest) (*GetObjectResponse, error)
}

type ObjectDeleter interface {
	DeleteObject(ctx context.Context, req *DeleteObjectRequest) error
	BatchDeleteObject(ctx context.Context, req *BatchDeleteObjectRequest) error
}

type ObjectUploader interface {
	UploadObject(ctx context.Context, req *UploadObjectRequest) error
}

type Storage interface {
	Provider
	ObjectGetter
	ObjectDeleter
	ObjectUploader

	PresignedPostPolicy(ctx context.Context, req *PresignedPostPolicyRequest) (*PresignedPostPolicyResponse, error)

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

	Inline bool

	Attachment bool

	AttachmentFilename string

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

	Body []byte

	BodyReader io.Reader

	ContentType string

	Metadata map[string]string
}

type UploadObjectResponse struct {
	Url string
}

type Config struct {
	Endpoint  string `toml:"endpoint"   json:"endpoint"`
	Region    string `toml:"region"     json:"region"`
	Bucket    string `toml:"bucket"     json:"bucket"`
	AccessKey string `toml:"access_key" json:"access_key"`
	SecretKey string `toml:"secret_key" json:"secret_key"`
	Secure    bool   `toml:"secure"     json:"secure"`

	PresignExpiry time.Duration `toml:"presign_expiry" json:"presign_expiry"`

	Extra map[string]string `toml:"extra" json:"extra"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return stderrors.New("endpoint is required")
	}
	if c.AccessKey == "" {
		return stderrors.New("access_key is required")
	}
	if c.SecretKey == "" {
		return stderrors.New("secret_key is required")
	}
	if c.Bucket == "" {
		return stderrors.New("bucket is required")
	}

	if c.PresignExpiry <= 0 {
		c.PresignExpiry = 15 * time.Minute
	}

	return nil
}
