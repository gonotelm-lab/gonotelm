package adapter

import (
	"context"
	"io"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

var ErrObjectNotFound = errors.ErrNoRecord.Msg("object not found")

// StoreKeyFactory 把"对象路径 + 可见性"补成 StoreKey（补齐桶名），业务侧不接触桶名。
type StoreKeyFactory interface {
	New(path string, isPublic bool) (valobj.StoreKey, error)
}

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
}

type UploadOptions struct {
	ContentType   string
	ContentLength int64
	Filename      string
	Md5           string
}

type PresignUploadResult struct {
	Method  string
	Url     string
	Forms   map[string]string
	Headers map[string]string
}

type ObjectGetter interface {
	// 返回 ErrObjectNotFound 表示对象不存在
	GetObject(ctx context.Context, key valobj.StoreKey) ([]byte, *ObjectInfo, error)
	GetPartialObject(ctx context.Context, key valobj.StoreKey, offset int64, length int64) ([]byte, *ObjectInfo, error)
}

type ObjectPutter interface {
	Upload(ctx context.Context, key valobj.StoreKey, content []byte, contentType string) error
	// UploadReader 流式上传，不关闭 src
	UploadReader(ctx context.Context, key valobj.StoreKey, contentType string, src io.Reader) error
	PresignUpload(ctx context.Context, key valobj.StoreKey, opts *UploadOptions) (*PresignUploadResult, error)
}

type ObjectDeleter interface {
	DeleteObject(ctx context.Context, key valobj.StoreKey) error
	BatchDeleteObject(ctx context.Context, keys []valobj.StoreKey) error
}

// ObjectPublicURLer 只需要"公有读对象的直链"能力的调用方依赖它即可。
type ObjectPublicURLer interface {
	// PublicURL 返回无需凭据即可访问的 URL；对象不是公有读（StoreKey.IsPublic=false）时返回错误。
	PublicURL(ctx context.Context, key valobj.StoreKey) (string, error)
}

// ObjectStore 是业务侧唯一的对象读写入口；实现内部按 StoreKey.Bucket 路由。
type ObjectStore interface {
	ObjectGetter
	ObjectPutter
	ObjectDeleter
	ObjectPublicURLer

	PresignGet(ctx context.Context, key valobj.StoreKey) (string, error)
	CheckExist(ctx context.Context, key valobj.StoreKey) (bool, error)
}
